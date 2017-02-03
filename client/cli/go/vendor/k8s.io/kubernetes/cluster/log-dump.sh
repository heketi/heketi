#!/bin/bash

# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Call this to dump all master and node logs into the folder specified in $1
# (defaults to _artifacts). Only works if the provider supports SSH.

set -o errexit
set -o nounset
set -o pipefail

readonly report_dir="${1:-_artifacts}"
# Enable LOG_DUMP_USE_KUBECTL to dump logs from a running cluster. In
# this mode, this script is standalone and doesn't use any of the bash
# provider infrastructure. Instead, the cluster is expected to have
# one node with the `kube-apiserver` image that we assume to be the
# master, and the LOG_DUMP_SSH_KEY and LOG_DUMP_SSH_USER variables
# must be set for auth.
readonly use_kubectl="${LOG_DUMP_USE_KUBECTL:-}"

readonly master_ssh_supported_providers="gce aws kubemark"
readonly node_ssh_supported_providers="gce gke aws"

readonly master_logfiles="kube-apiserver kube-scheduler kube-controller-manager etcd glbc cluster-autoscaler"
readonly node_logfiles="kube-proxy"
readonly aws_logfiles="cloud-init-output"
readonly gce_logfiles="startupscript"
readonly kern_logfile="kern"
readonly initd_logfiles="docker"
readonly supervisord_logfiles="kubelet supervisor/supervisord supervisor/kubelet-stdout supervisor/kubelet-stderr supervisor/docker-stdout supervisor/docker-stderr"

# Limit the number of concurrent node connections so that we don't run out of
# file descriptors for large clusters.
readonly max_scp_processes=25

# This template spits out the external IPs and images for each node in the cluster in a format like so:
# 52.32.7.85 gcr.io/google_containers/kube-apiserver:1355c18c32d7bef16125120bce194fad gcr.io/google_containers/kube-controller-manager:46365cdd8d28b8207950c3c21d1f3900 [...]
readonly ips_and_images='{range .items[*]}{@.status.addresses[?(@.type == "ExternalIP")].address} {@.status.images[*].names[*]}{"\n"}{end}'

function setup() {
  if [[ -z "${use_kubectl}" ]]; then
    KUBE_ROOT=$(dirname "${BASH_SOURCE}")/..
    : ${KUBE_CONFIG_FILE:="config-test.sh"}
    source "${KUBE_ROOT}/cluster/kube-util.sh"
    detect-project &> /dev/null
  elif [[ -z "${LOG_DUMP_SSH_KEY:-}" ]]; then
    echo "LOG_DUMP_SSH_KEY not set, but required by LOG_DUMP_USE_KUBECTL"
    exit 1
  elif [[ -z "${LOG_DUMP_SSH_USER:-}" ]]; then
    echo "LOG_DUMP_SSH_USER not set, but required by LOG_DUMP_USE_KUBECTL"
    exit 1
  fi
}

function log-dump-ssh() {
  if [[ -z "${use_kubectl}" ]]; then
    ssh-to-node "$@"
    return
  fi

  local host="$1"
  local cmd="$2"

  ssh -oLogLevel=quiet -oConnectTimeout=30 -oStrictHostKeyChecking=no -i "${LOG_DUMP_SSH_KEY}" "${LOG_DUMP_SSH_USER}@${host}" "${cmd}"
}

# Copy all files /var/log/{$3}.log on node $1 into local dir $2.
# $3 should be a space-separated string of files.
# This function shouldn't ever trigger errexit, but doesn't block stderr.
function copy-logs-from-node() {
    local -r node="${1}"
    local -r dir="${2}"
    local files=( ${3} )
    # Append ".log*"
    # The * at the end is needed to also copy rotated logs (which happens
    # in large clusters and long runs).
    files=( "${files[@]/%/.log*}" )
    # Prepend "/var/log/"
    files=( "${files[@]/#/\/var\/log\/}" )
    # Comma delimit (even the singleton, or scp does the wrong thing), surround by braces.
    local -r scp_files="{$(printf "%s," "${files[@]}")}"

    if [[ -n "${use_kubectl}" ]]; then
      scp -oLogLevel=quiet -oConnectTimeout=30 -oStrictHostKeyChecking=no -i "${LOG_DUMP_SSH_KEY}" "${LOG_DUMP_SSH_USER}@${node}:${scp_files}" "${dir}" > /dev/null || true
    else
      case "${KUBERNETES_PROVIDER}" in
        gce|gke|kubemark)
          gcloud compute copy-files --project "${PROJECT}" --zone "${ZONE}" "${node}:${scp_files}" "${dir}" > /dev/null || true
          ;;
        aws)
          local ip=$(get_ssh_hostname "${node}")
          scp -oLogLevel=quiet -oConnectTimeout=30 -oStrictHostKeyChecking=no -i "${AWS_SSH_KEY}" "${SSH_USER}@${ip}:${scp_files}" "${dir}" > /dev/null || true
          ;;
      esac
    fi
}

# Save logs for node $1 into directory $2. Pass in any non-common files in $3.
# $3 should be a space-separated list of files.
# This function shouldn't ever trigger errexit
function save-logs() {
    local -r node_name="${1}"
    local -r dir="${2}"
    local files="${3}"
    if [[ -n "${use_kubectl}" ]]; then
      if [[ -n "${LOG_DUMP_SAVE_LOGS:-}" ]]; then
        files="${files} ${LOG_DUMP_SAVE_LOGS:-}"
      fi
    else
      case "${KUBERNETES_PROVIDER}" in
        gce|gke|kubemark)
          files="${files} ${gce_logfiles}"
          ;;
        aws)
          files="${files} ${aws_logfiles}"
          ;;
      esac
    fi

    if log-dump-ssh "${node_name}" "sudo systemctl status kubelet.service" &> /dev/null; then
        log-dump-ssh "${node_name}" "sudo journalctl --output=cat -u kubelet.service" > "${dir}/kubelet.log" || true
        log-dump-ssh "${node_name}" "sudo journalctl --output=cat -u docker.service" > "${dir}/docker.log" || true
        log-dump-ssh "${node_name}" "sudo journalctl --output=cat -k" > "${dir}/kern.log" || true
    else
        files="${kern_logfile} ${files} ${initd_logfiles} ${supervisord_logfiles}"
    fi
    echo "Copying '${files}' from ${node_name}"
    copy-logs-from-node "${node_name}" "${dir}" "${files}"
}

function kubectl-guess-master() {
  kubectl get node -ojsonpath --template="${ips_and_images}" | grep kube-apiserver | cut -f1 -d" "
}

function kubectl-guess-nodes() {
  kubectl get node -ojsonpath --template="${ips_and_images}" | grep -v kube-apiserver | cut -f1 -d" "
}

function dump_master() {
  local master_name
  if [[ -n "${use_kubectl}" ]]; then
    master_name=$(kubectl-guess-master)
  elif [[ ! "${master_ssh_supported_providers}" =~ "${KUBERNETES_PROVIDER}" ]]; then
    echo "Master SSH not supported for ${KUBERNETES_PROVIDER}"
    return
  else
    if ! (detect-master &> /dev/null); then
      echo "Master not detected. Is the cluster up?"
      return
    fi
    master_name="${MASTER_NAME}"
  fi

  readonly master_dir="${report_dir}/${master_name}"
  mkdir -p "${master_dir}"
  save-logs "${master_name}" "${master_dir}" "${master_logfiles}"
}

function dump_nodes() {
  local node_names
  if [[ -n "${use_kubectl}" ]]; then
    node_names=( $(kubectl-guess-nodes) )
  elif [[ ! "${node_ssh_supported_providers}" =~ "${KUBERNETES_PROVIDER}" ]]; then
    echo "Node SSH not supported for ${KUBERNETES_PROVIDER}"
    return
  else
    detect-node-names &> /dev/null
    if [[ "${#NODE_NAMES[@]}" -eq 0 ]]; then
      echo "Nodes not detected. Is the cluster up?"
      return
    fi
    node_names=( "${NODE_NAMES[@]}" )
  fi

  proc=${max_scp_processes}
  for node_name in "${node_names[@]}"; do
    node_dir="${report_dir}/${node_name}"
    mkdir -p "${node_dir}"
    # Save logs in the background. This speeds up things when there are
    # many nodes.
    save-logs "${node_name}" "${node_dir}" "${node_logfiles}" &

    # We don't want to run more than ${max_scp_processes} at a time, so
    # wait once we hit that many nodes. This isn't ideal, since one might
    # take much longer than the others, but it should help.
    proc=$((proc - 1))
    if [[ proc -eq 0 ]]; then
      proc=${max_scp_processes}
      wait
    fi
  done
  # Wait for any remaining processes.
  if [[ proc -gt 0 && proc -lt ${max_scp_processes} ]]; then
    wait
  fi
}

setup
echo "Dumping master and node logs to ${report_dir}"
dump_master
dump_nodes
