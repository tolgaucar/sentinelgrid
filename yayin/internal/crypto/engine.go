// Package crypto — AES-256-GCM uçtan uca şifreleme.
//
// Askeri haberleşmede veri gizliliği için AES-256-GCM kullanır.
// Anahtar döndürme (rotation) destekler; eski anahtarlar belirli
// süre saklanarak geçiş döneminde çözümleme yapılabilir.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// KeyInfo bir şifreleme anahtarının meta bilgisi.
type KeyInfo struct {
	ID        uint32    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"`
}

// Engine AES-256-GCM şifreleme motoru.
type Engine struct {
	mu        sync.RWMutex
	activeKey []byte       // 32 byte AES-256 anahtarı
	activeID  uint32       // Aktif anahtar kimliği
	oldKeys   map[uint32][]byte // Eski anahtarlar (çözümleme için)
	keyInfo   []KeyInfo
	maxOld    int

	stats EngineStats
}

// EngineStats şifreleme istatistikleri.
type EngineStats struct {
	TotalEncrypted int64 `json:"total_encrypted"`
	TotalDecrypted int64 `json:"total_decrypted"`
	TotalErrors    int64 `json:"total_errors"`
	KeyRotations   int64 `json:"key_rotations"`
	BytesEncrypted int64 `json:"bytes_encrypted"`
	BytesDecrypted int64 `json:"bytes_decrypted"`
}

// NewEngine yeni şifreleme motoru oluşturur.
func NewEngine(maxOldKeys int) (*Engine, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("anahtar üretim hatası: %w", err)
	}

	e := &Engine{
		activeKey: key,
		activeID:  1,
		oldKeys:   make(map[uint32][]byte),
		maxOld:    maxOldKeys,
		keyInfo: []KeyInfo{
			{ID: 1, CreatedAt: time.Now(), Active: true},
		},
	}
	return e, nil
}

// Encrypt veriyi AES-256-GCM ile şifreler.
// Çıktı formatı: [4B keyID][nonce][ciphertext+tag]
func (e *Engine) Encrypt(plaintext []byte) ([]byte, error) {
	e.mu.RLock()
	key := e.activeKey
	keyID := e.activeID
	e.mu.RUnlock()

	block, err := aes.NewCipher(key)
	if err != nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("AES cipher hatası: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("GCM hatası: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("nonce üretim hatası: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Format: [keyID:4][nonce:12][ciphertext+tag]
	result := make([]byte, 4+len(nonce)+len(ciphertext))
	binary.BigEndian.PutUint32(result[:4], keyID)
	copy(result[4:4+len(nonce)], nonce)
	copy(result[4+len(nonce):], ciphertext)

	atomic.AddInt64(&e.stats.TotalEncrypted, 1)
	atomic.AddInt64(&e.stats.BytesEncrypted, int64(len(plaintext)))
	return result, nil
}

// Decrypt şifreli veriyi çözer. Anahtar ID'sine göre eski anahtarlarla da dener.
func (e *Engine) Decrypt(data []byte) ([]byte, error) {
	if len(data) < 4+12 { // keyID + minimum nonce
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("veri çok kısa: %d byte", len(data))
	}

	keyID := binary.BigEndian.Uint32(data[:4])

	e.mu.RLock()
	var key []byte
	if keyID == e.activeID {
		key = e.activeKey
	} else {
		key = e.oldKeys[keyID]
	}
	e.mu.RUnlock()

	if key == nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("anahtar bulunamadı: ID=%d", keyID)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("AES cipher hatası: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("GCM hatası: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < 4+nonceSize {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("veri çok kısa nonce için: %d byte", len(data))
	}

	nonce := data[4 : 4+nonceSize]
	ciphertext := data[4+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		atomic.AddInt64(&e.stats.TotalErrors, 1)
		return nil, fmt.Errorf("çözümleme hatası (bütünlük kontrolü başarısız): %w", err)
	}

	atomic.AddInt64(&e.stats.TotalDecrypted, 1)
	atomic.AddInt64(&e.stats.BytesDecrypted, int64(len(plaintext)))
	return plaintext, nil
}

// RotateKey aktif anahtarı döndürür. Eski anahtar çözümleme için saklanır.
func (e *Engine) RotateKey() error {
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return fmt.Errorf("yeni anahtar üretim hatası: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Eski anahtarı sakla
	e.oldKeys[e.activeID] = e.activeKey

	// Limiti aştıysa en eski anahtarı sil
	if len(e.oldKeys) > e.maxOld {
		var minID uint32 = ^uint32(0)
		for id := range e.oldKeys {
			if id < minID {
				minID = id
			}
		}
		delete(e.oldKeys, minID)
	}

	// Mevcut aktif anahtarın info'sunu güncelle
	for i := range e.keyInfo {
		if e.keyInfo[i].ID == e.activeID {
			e.keyInfo[i].Active = false
			break
		}
	}

	e.activeID++
	e.activeKey = newKey
	e.keyInfo = append(e.keyInfo, KeyInfo{
		ID:        e.activeID,
		CreatedAt: time.Now(),
		Active:    true,
	})

	atomic.AddInt64(&e.stats.KeyRotations, 1)
	return nil
}

// Stats istatistikleri döndürür.
func (e *Engine) Stats() EngineStats {
	return EngineStats{
		TotalEncrypted: atomic.LoadInt64(&e.stats.TotalEncrypted),
		TotalDecrypted: atomic.LoadInt64(&e.stats.TotalDecrypted),
		TotalErrors:    atomic.LoadInt64(&e.stats.TotalErrors),
		KeyRotations:   atomic.LoadInt64(&e.stats.KeyRotations),
		BytesEncrypted: atomic.LoadInt64(&e.stats.BytesEncrypted),
		BytesDecrypted: atomic.LoadInt64(&e.stats.BytesDecrypted),
	}
}

// ActiveKeyID aktif anahtar kimliğini döndürür.
func (e *Engine) ActiveKeyID() uint32 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.activeID
}

// KeyCount toplam anahtar sayısını döndürür (aktif + eski).
func (e *Engine) KeyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return 1 + len(e.oldKeys)
}

// Keys anahtar bilgilerini döndürür.
func (e *Engine) Keys() []KeyInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]KeyInfo, len(e.keyInfo))
	copy(result, e.keyInfo)
	return result
}
