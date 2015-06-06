package mock

type Volume struct {
	name, mount string
	size        uint64
}

type Volumes []Volume
