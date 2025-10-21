package backend

import (
	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
)

// PStor implements persistent storage.Stor
type PStor struct {
	kbv1.UnimplementedStorServer
	// storage.Store
}

// func NewPStor(storURL string) *PStor {
// 	s, _ := storage.NewStore(storURL)
// 	return s
// }

// func (ps *PStor) Save(i models.Item) error {
// 	return nil
// }

// func (ps *PStor) Close() error {
// 	return nil
// }
