package store

import "errors"

var (
	ErrKeyNotFound = errors.New("kütük: anahtar bulunamadı")
	ErrKeyExpired  = errors.New("kütük: anahtar süresi dolmuş")
	ErrStoreFull   = errors.New("kütük: depo kapasitesi dolu")
)
