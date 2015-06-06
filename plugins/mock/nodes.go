package mock

type Node struct {
	ip       string // May need to be changed to use net.IPaddr later
	hostname string
	volumes  Volumes
	storages Storages
}
