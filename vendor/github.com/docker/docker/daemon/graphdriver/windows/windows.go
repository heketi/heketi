//+build windows

package windows

import (
	"bufio"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/archive/tar"
	"github.com/Microsoft/go-winio/backuptar"
	"github.com/Microsoft/hcsshim"
	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/vbatts/tar-split/tar/storage"
)

// init registers the windows graph drivers to the register.
func init() {
	graphdriver.Register("windowsfilter", InitFilter)
	graphdriver.Register("windowsdiff", InitDiff)
}

const (
	// diffDriver is an hcsshim driver type
	diffDriver = iota
	// filterDriver is an hcsshim driver type
	filterDriver
)

// Driver represents a windows graph driver.
type Driver struct {
	// info stores the shim driver information
	info hcsshim.DriverInfo
}

var _ graphdriver.DiffGetterDriver = &Driver{}

// InitFilter returns a new Windows storage filter driver.
func InitFilter(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("WindowsGraphDriver InitFilter at %s", home)
	d := &Driver{
		info: hcsshim.DriverInfo{
			HomeDir: home,
			Flavour: filterDriver,
		},
	}
	return d, nil
}

// InitDiff returns a new Windows differencing disk driver.
func InitDiff(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("WindowsGraphDriver InitDiff at %s", home)
	d := &Driver{
		info: hcsshim.DriverInfo{
			HomeDir: home,
			Flavour: diffDriver,
		},
	}
	return d, nil
}

// String returns the string representation of a driver.
func (d *Driver) String() string {
	switch d.info.Flavour {
	case diffDriver:
		return "windowsdiff"
	case filterDriver:
		return "windowsfilter"
	default:
		return "Unknown driver flavour"
	}
}

// Status returns the status of the driver.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"Windows", ""},
	}
}

// Exists returns true if the given id is registered with this driver.
func (d *Driver) Exists(id string) bool {
	rID, err := d.resolveID(id)
	if err != nil {
		return false
	}
	result, err := hcsshim.LayerExists(d.info, rID)
	if err != nil {
		return false
	}
	return result
}

// Create creates a new layer with the given id.
func (d *Driver) Create(id, parent, mountLabel string) error {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return err
	}

	parentChain, err := d.getLayerChain(rPId)
	if err != nil {
		return err
	}

	var layerChain []string

	parentIsInit := strings.HasSuffix(rPId, "-init")

	if !parentIsInit && rPId != "" {
		parentPath, err := hcsshim.GetLayerMountPath(d.info, rPId)
		if err != nil {
			return err
		}
		layerChain = []string{parentPath}
	}

	layerChain = append(layerChain, parentChain...)

	if parentIsInit {
		if len(layerChain) == 0 {
			return fmt.Errorf("Cannot create a read/write layer without a parent layer.")
		}
		if err := hcsshim.CreateSandboxLayer(d.info, id, layerChain[0], layerChain); err != nil {
			return err
		}
	} else {
		if err := hcsshim.CreateLayer(d.info, id, rPId); err != nil {
			return err
		}
	}

	if _, err := os.Lstat(d.dir(parent)); err != nil {
		if err2 := hcsshim.DestroyLayer(d.info, id); err2 != nil {
			logrus.Warnf("Failed to DestroyLayer %s: %s", id, err2)
		}
		return fmt.Errorf("Cannot create layer with missing parent %s: %s", parent, err)
	}

	if err := d.setLayerChain(id, layerChain); err != nil {
		if err2 := hcsshim.DestroyLayer(d.info, id); err2 != nil {
			logrus.Warnf("Failed to DestroyLayer %s: %s", id, err2)
		}
		return err
	}

	return nil
}

// dir returns the absolute path to the layer.
func (d *Driver) dir(id string) string {
	return filepath.Join(d.info.HomeDir, filepath.Base(id))
}

// Remove unmounts and removes the dir information.
func (d *Driver) Remove(id string) error {
	rID, err := d.resolveID(id)
	if err != nil {
		return err
	}
	os.RemoveAll(filepath.Join(d.info.HomeDir, "sysfile-backups", rID)) // ok to fail
	return hcsshim.DestroyLayer(d.info, rID)
}

// Get returns the rootfs path for the id. This will mount the dir at it's given path.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	logrus.Debugf("WindowsGraphDriver Get() id %s mountLabel %s", id, mountLabel)
	var dir string

	rID, err := d.resolveID(id)
	if err != nil {
		return "", err
	}

	// Getting the layer paths must be done outside of the lock.
	layerChain, err := d.getLayerChain(rID)
	if err != nil {
		return "", err
	}

	if err := hcsshim.ActivateLayer(d.info, rID); err != nil {
		return "", err
	}
	if err := hcsshim.PrepareLayer(d.info, rID, layerChain); err != nil {
		if err2 := hcsshim.DeactivateLayer(d.info, rID); err2 != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", id, err)
		}
		return "", err
	}

	mountPath, err := hcsshim.GetLayerMountPath(d.info, rID)
	if err != nil {
		if err2 := hcsshim.DeactivateLayer(d.info, rID); err2 != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", id, err)
		}
		return "", err
	}

	// If the layer has a mount path, use that. Otherwise, use the
	// folder path.
	if mountPath != "" {
		dir = mountPath
	} else {
		dir = d.dir(id)
	}

	return dir, nil
}

// Put adds a new layer to the driver.
func (d *Driver) Put(id string) error {
	logrus.Debugf("WindowsGraphDriver Put() id %s", id)

	rID, err := d.resolveID(id)
	if err != nil {
		return err
	}

	if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
		return err
	}
	return hcsshim.DeactivateLayer(d.info, rID)
}

// Cleanup ensures the information the driver stores is properly removed.
func (d *Driver) Cleanup() error {
	return nil
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
// The layer should be mounted when calling this function
func (d *Driver) Diff(id, parent string) (_ archive.Archive, err error) {
	rID, err := d.resolveID(id)
	if err != nil {
		return
	}

	layerChain, err := d.getLayerChain(rID)
	if err != nil {
		return
	}

	// this is assuming that the layer is unmounted
	if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
		return nil, err
	}
	defer func() {
		if err := hcsshim.PrepareLayer(d.info, rID, layerChain); err != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", rID, err)
		}
	}()

	arch, err := d.exportLayer(rID, layerChain)
	if err != nil {
		return
	}
	return ioutils.NewReadCloserWrapper(arch, func() error {
		return arch.Close()
	}), nil
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
// The layer should be mounted when calling this function
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	rID, err := d.resolveID(id)
	if err != nil {
		return nil, err
	}
	parentChain, err := d.getLayerChain(rID)
	if err != nil {
		return nil, err
	}

	// this is assuming that the layer is unmounted
	if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
		return nil, err
	}
	defer func() {
		if err := hcsshim.PrepareLayer(d.info, rID, parentChain); err != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", rID, err)
		}
	}()

	r, err := hcsshim.NewLayerReader(d.info, id, parentChain)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var changes []archive.Change
	for {
		name, _, fileInfo, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name = filepath.ToSlash(name)
		if fileInfo == nil {
			changes = append(changes, archive.Change{name, archive.ChangeDelete})
		} else {
			// Currently there is no way to tell between an add and a modify.
			changes = append(changes, archive.Change{name, archive.ChangeModify})
		}
	}
	return changes, nil
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
// The layer should not be mounted when calling this function
func (d *Driver) ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error) {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return
	}

	if d.info.Flavour == diffDriver {
		start := time.Now().UTC()
		logrus.Debugf("WindowsGraphDriver ApplyDiff: Start untar layer")
		destination := d.dir(id)
		destination = filepath.Dir(destination)
		if size, err = chrootarchive.ApplyUncompressedLayer(destination, diff, nil); err != nil {
			return
		}
		logrus.Debugf("WindowsGraphDriver ApplyDiff: Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

		return
	}

	parentChain, err := d.getLayerChain(rPId)
	if err != nil {
		return
	}
	parentPath, err := hcsshim.GetLayerMountPath(d.info, rPId)
	if err != nil {
		return
	}
	layerChain := []string{parentPath}
	layerChain = append(layerChain, parentChain...)

	if size, err = d.importLayer(id, diff, layerChain); err != nil {
		return
	}

	if err = d.setLayerChain(id, layerChain); err != nil {
		return
	}

	return
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return
	}

	changes, err := d.Changes(id, rPId)
	if err != nil {
		return
	}

	layerFs, err := d.Get(id, "")
	if err != nil {
		return
	}
	defer d.Put(id)

	return archive.ChangesSize(layerFs, changes), nil
}

// CustomImageInfo is the object returned by the driver describing the base
// image.
type CustomImageInfo struct {
	ID          string
	Name        string
	Version     string
	Path        string
	Size        int64
	CreatedTime time.Time
}

// GetCustomImageInfos returns the image infos for window specific
// base images which should always be present.
func (d *Driver) GetCustomImageInfos() ([]CustomImageInfo, error) {
	strData, err := hcsshim.GetSharedBaseImages()
	if err != nil {
		return nil, fmt.Errorf("Failed to restore base images: %s", err)
	}

	type customImageInfoList struct {
		Images []CustomImageInfo
	}

	var infoData customImageInfoList

	if err = json.Unmarshal([]byte(strData), &infoData); err != nil {
		err = fmt.Errorf("JSON unmarshal returned error=%s", err)
		logrus.Error(err)
		return nil, err
	}

	var images []CustomImageInfo

	for _, imageData := range infoData.Images {
		folderName := filepath.Base(imageData.Path)

		// Use crypto hash of the foldername to generate a docker style id.
		h := sha512.Sum384([]byte(folderName))
		id := fmt.Sprintf("%x", h[:32])

		if err := d.Create(id, "", ""); err != nil {
			return nil, err
		}
		// Create the alternate ID file.
		if err := d.setID(id, folderName); err != nil {
			return nil, err
		}

		imageData.ID = id
		images = append(images, imageData)
	}

	return images, nil
}

// GetMetadata returns custom driver information.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	m := make(map[string]string)
	m["dir"] = d.dir(id)
	return m, nil
}

func writeTarFromLayer(r hcsshim.LayerReader, w io.Writer) error {
	t := tar.NewWriter(w)
	for {
		name, size, fileInfo, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if fileInfo == nil {
			// Write a whiteout file.
			hdr := &tar.Header{
				Name: filepath.ToSlash(filepath.Join(filepath.Dir(name), archive.WhiteoutPrefix+filepath.Base(name))),
			}
			err := t.WriteHeader(hdr)
			if err != nil {
				return err
			}
		} else {
			err = backuptar.WriteTarFileFromBackupStream(t, r, name, size, fileInfo)
			if err != nil {
				return err
			}
		}
	}
	return t.Close()
}

// exportLayer generates an archive from a layer based on the given ID.
func (d *Driver) exportLayer(id string, parentLayerPaths []string) (archive.Archive, error) {
	if hcsshim.IsTP4() {
		// Export in TP4 format to maintain compatibility with existing images and
		// because ExportLayer is somewhat broken on TP4 and can't work with the new
		// scheme.
		tempFolder, err := ioutil.TempDir("", "hcs")
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(tempFolder)
			}
		}()

		if err = hcsshim.ExportLayer(d.info, id, tempFolder, parentLayerPaths); err != nil {
			return nil, err
		}
		archive, err := archive.Tar(tempFolder, archive.Uncompressed)
		if err != nil {
			return nil, err
		}
		return ioutils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			os.RemoveAll(tempFolder)
			return err
		}), nil
	}

	var r hcsshim.LayerReader
	r, err := hcsshim.NewLayerReader(d.info, id, parentLayerPaths)
	if err != nil {
		return nil, err
	}

	archive, w := io.Pipe()
	go func() {
		err := writeTarFromLayer(r, w)
		cerr := r.Close()
		if err == nil {
			err = cerr
		}
		w.CloseWithError(err)
	}()

	return archive, nil
}

func writeLayerFromTar(r archive.Reader, w hcsshim.LayerWriter) (int64, error) {
	t := tar.NewReader(r)
	hdr, err := t.Next()
	totalSize := int64(0)
	buf := bufio.NewWriter(nil)
	for err == nil {
		base := path.Base(hdr.Name)
		if strings.HasPrefix(base, archive.WhiteoutPrefix) {
			name := path.Join(path.Dir(hdr.Name), base[len(archive.WhiteoutPrefix):])
			err = w.Remove(filepath.FromSlash(name))
			if err != nil {
				return 0, err
			}
			hdr, err = t.Next()
		} else {
			var (
				name     string
				size     int64
				fileInfo *winio.FileBasicInfo
			)
			name, size, fileInfo, err = backuptar.FileInfoFromHeader(hdr)
			if err != nil {
				return 0, err
			}
			err = w.Add(filepath.FromSlash(name), fileInfo)
			if err != nil {
				return 0, err
			}
			buf.Reset(w)
			hdr, err = backuptar.WriteBackupStreamFromTarFile(buf, t, hdr)
			ferr := buf.Flush()
			if ferr != nil {
				err = ferr
			}
			totalSize += size
		}
	}
	if err != io.EOF {
		return 0, err
	}
	return totalSize, nil
}

// importLayer adds a new layer to the tag and graph store based on the given data.
func (d *Driver) importLayer(id string, layerData archive.Reader, parentLayerPaths []string) (size int64, err error) {
	if hcsshim.IsTP4() {
		// Import from TP4 format to maintain compatibility with existing images.
		var tempFolder string
		tempFolder, err = ioutil.TempDir("", "hcs")
		if err != nil {
			return
		}
		defer os.RemoveAll(tempFolder)

		if size, err = chrootarchive.ApplyLayer(tempFolder, layerData); err != nil {
			return
		}
		if err = hcsshim.ImportLayer(d.info, id, tempFolder, parentLayerPaths); err != nil {
			return
		}
		return
	}

	var w hcsshim.LayerWriter
	w, err = hcsshim.NewLayerWriter(d.info, id, parentLayerPaths)
	if err != nil {
		return
	}

	size, err = writeLayerFromTar(layerData, w)
	if err != nil {
		w.Close()
		return
	}
	err = w.Close()
	if err != nil {
		return
	}
	return
}

// resolveID computes the layerID information based on the given id.
func (d *Driver) resolveID(id string) (string, error) {
	content, err := ioutil.ReadFile(filepath.Join(d.dir(id), "layerID"))
	if os.IsNotExist(err) {
		return id, nil
	} else if err != nil {
		return "", err
	}
	return string(content), nil
}

// setID stores the layerId in disk.
func (d *Driver) setID(id, altID string) error {
	err := ioutil.WriteFile(filepath.Join(d.dir(id), "layerId"), []byte(altID), 0600)
	if err != nil {
		return err
	}
	return nil
}

// getLayerChain returns the layer chain information.
func (d *Driver) getLayerChain(id string) ([]string, error) {
	jPath := filepath.Join(d.dir(id), "layerchain.json")
	content, err := ioutil.ReadFile(jPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("Unable to read layerchain file - %s", err)
	}

	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall layerchain json - %s", err)
	}

	return layerChain, nil
}

// setLayerChain stores the layer chain information in disk.
func (d *Driver) setLayerChain(id string, chain []string) error {
	content, err := json.Marshal(&chain)
	if err != nil {
		return fmt.Errorf("Failed to marshall layerchain json - %s", err)
	}

	jPath := filepath.Join(d.dir(id), "layerchain.json")
	err = ioutil.WriteFile(jPath, content, 0600)
	if err != nil {
		return fmt.Errorf("Unable to write layerchain file - %s", err)
	}

	return nil
}

type fileGetCloserWithBackupPrivileges struct {
	path string
}

func (fg *fileGetCloserWithBackupPrivileges) Get(filename string) (io.ReadCloser, error) {
	var f *os.File
	// Open the file while holding the Windows backup privilege. This ensures that the
	// file can be opened even if the caller does not actually have access to it according
	// to the security descriptor.
	err := winio.RunWithPrivilege(winio.SeBackupPrivilege, func() error {
		path := filepath.Join(fg.path, filename)
		p, err := syscall.UTF16FromString(path)
		if err != nil {
			return err
		}
		h, err := syscall.CreateFile(&p[0], syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			return &os.PathError{Op: "open", Path: path, Err: err}
		}
		f = os.NewFile(uintptr(h), path)
		return nil
	})
	return f, err
}

func (fg *fileGetCloserWithBackupPrivileges) Close() error {
	return nil
}

type fileGetDestroyCloser struct {
	storage.FileGetter
	path string
}

func (f *fileGetDestroyCloser) Close() error {
	// TODO: activate layers and release here?
	return os.RemoveAll(f.path)
}

// DiffGetter returns a FileGetCloser that can read files from the directory that
// contains files for the layer differences. Used for direct access for tar-split.
func (d *Driver) DiffGetter(id string) (graphdriver.FileGetCloser, error) {
	id, err := d.resolveID(id)
	if err != nil {
		return nil, err
	}

	if hcsshim.IsTP4() {
		// The export format for TP4 is different from the contents of the layer, so
		// fall back to exporting the layer and getting file contents from there.
		layerChain, err := d.getLayerChain(id)
		if err != nil {
			return nil, err
		}

		var tempFolder string
		tempFolder, err = ioutil.TempDir("", "hcs")
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(tempFolder)
			}
		}()

		if err = hcsshim.ExportLayer(d.info, id, tempFolder, layerChain); err != nil {
			return nil, err
		}

		return &fileGetDestroyCloser{storage.NewPathFileGetter(tempFolder), tempFolder}, nil
	}

	return &fileGetCloserWithBackupPrivileges{d.dir(id)}, nil
}
