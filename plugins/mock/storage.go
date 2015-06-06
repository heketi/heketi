package mock

type Storage struct {
	name              string
	free, used, total uint64
}

type Storages []Storage
