// Package fec — İleri Hata Düzeltme (Forward Error Correction).
//
// Paket kayıplarına dayanıklılık sağlar. Veriyi bloklara ayırır,
// her N veri bloğuna 1 XOR parite bloğu ekler. Kayıp blok
// pariteden kurtarılabilir. Askeri haberleşmede karışık/gürültülü
// kanallar için kritik önem taşır.
package fec

import (
	"fmt"
	"sort"
	"sync/atomic"
)

// Block FEC işlenmiş bir veri bloğu.
type Block struct {
	Index    int    `json:"index"`
	IsParity bool   `json:"is_parity"`
	Data     []byte `json:"data"`
	GroupID  int    `json:"group_id"`
}

// Encoder FEC kodlayıcı.
type Encoder struct {
	dataShards int // Grup başına veri blok sayısı
	blockSize  int // Blok boyutu
	stats      EncoderStats
}

// EncoderStats FEC istatistikleri.
type EncoderStats struct {
	TotalEncoded   int64 `json:"total_encoded"`
	TotalDecoded   int64 `json:"total_decoded"`
	TotalRecovered int64 `json:"total_recovered"`
	TotalErrors    int64 `json:"total_errors"`
	BlocksCreated  int64 `json:"blocks_created"`
	ParityCreated  int64 `json:"parity_created"`
}

// NewEncoder yeni FEC kodlayıcı oluşturur.
// dataShards: Her gruba kaç veri bloğu (ör: 4 → her 4 blokta 1 parite).
// blockSize: Her bloğun byte boyutu.
func NewEncoder(dataShards, blockSize int) (*Encoder, error) {
	if dataShards < 2 {
		return nil, fmt.Errorf("dataShards en az 2 olmalı, verilen: %d", dataShards)
	}
	if blockSize < 1 {
		return nil, fmt.Errorf("blockSize en az 1 olmalı, verilen: %d", blockSize)
	}
	return &Encoder{
		dataShards: dataShards,
		blockSize:  blockSize,
	}, nil
}

// Encode veriyi bloklara ayırır ve parite blokları ekler.
func (e *Encoder) Encode(data []byte) []Block {
	if len(data) == 0 {
		return nil
	}

	// Veriyi bloklara böl (son blok padding'li)
	var blocks []Block
	groupID := 0
	groupCount := 0
	var parityBuf []byte

	for offset := 0; offset < len(data); offset += e.blockSize {
		end := offset + e.blockSize
		if end > len(data) {
			end = len(data)
		}

		blockData := make([]byte, e.blockSize)
		copy(blockData, data[offset:end])

		blocks = append(blocks, Block{
			Index:    len(blocks),
			IsParity: false,
			Data:     blockData,
			GroupID:  groupID,
		})

		// XOR parite hesapla
		if parityBuf == nil {
			parityBuf = make([]byte, e.blockSize)
			copy(parityBuf, blockData)
		} else {
			xorBytes(parityBuf, blockData)
		}

		groupCount++
		atomic.AddInt64(&e.stats.BlocksCreated, 1)

		if groupCount == e.dataShards {
			// Parite bloğu ekle
			parity := make([]byte, e.blockSize)
			copy(parity, parityBuf)
			blocks = append(blocks, Block{
				Index:    len(blocks),
				IsParity: true,
				Data:     parity,
				GroupID:  groupID,
			})

			atomic.AddInt64(&e.stats.ParityCreated, 1)
			groupID++
			groupCount = 0
			parityBuf = nil
		}
	}

	// Son grupta eksik blok varsa pariteyi yine de ekle
	if groupCount > 0 && parityBuf != nil {
		blocks = append(blocks, Block{
			Index:    len(blocks),
			IsParity: true,
			Data:     parityBuf,
			GroupID:  groupID,
		})
		atomic.AddInt64(&e.stats.ParityCreated, 1)
	}

	atomic.AddInt64(&e.stats.TotalEncoded, 1)
	return blocks
}

// Decode blokları orijinal veriye dönüştürür. Kayıp 1 blok kurtarılabilir.
// originalSize kayıp olmayan orijinal veri boyutu.
func (e *Encoder) Decode(blocks []Block, originalSize int) ([]byte, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("blok yok")
	}

	// Toplam veri bloğu sayısını orijinal boyuttan hesapla
	totalDataBlocks := (originalSize + e.blockSize - 1) / e.blockSize

	// Blokları gruplara ayır
	groups := make(map[int][]Block)
	for _, b := range blocks {
		groups[b.GroupID] = append(groups[b.GroupID], b)
	}

	// Her grupta eksik blokları kurtarmaya çalış
	var allData []Block
	for gid := 0; gid <= maxGroupID(blocks); gid++ {
		group := groups[gid]
		dataBlocks, parity := splitGroup(group)

		// Bu grubun beklenen veri bloğu sayısı
		startBlock := gid * e.dataShards
		endBlock := startBlock + e.dataShards
		if endBlock > totalDataBlocks {
			endBlock = totalDataBlocks
		}
		expectedData := endBlock - startBlock

		if len(dataBlocks) == expectedData {
			allData = append(allData, dataBlocks...)
		} else if len(dataBlocks) == expectedData-1 && parity != nil {
			// 1 eksik blok → pariteden kurtar
			recovered := e.recoverBlock(dataBlocks, parity, expectedData)
			if recovered != nil {
				allData = append(allData, dataBlocks...)
				allData = append(allData, *recovered)
				atomic.AddInt64(&e.stats.TotalRecovered, 1)
			} else {
				atomic.AddInt64(&e.stats.TotalErrors, 1)
				return nil, fmt.Errorf("grup %d: blok kurtarılamadı", gid)
			}
		} else {
			atomic.AddInt64(&e.stats.TotalErrors, 1)
			return nil, fmt.Errorf("grup %d: %d eksik blok (maks 1 kurtarılabilir)", gid, expectedData-len(dataBlocks))
		}
	}

	// Blokları index sırasına göre sırala
	sort.Slice(allData, func(i, j int) bool {
		return allData[i].Index < allData[j].Index
	})

	// Blokları birleştir
	result := make([]byte, 0, len(allData)*e.blockSize)
	for _, b := range allData {
		result = append(result, b.Data...)
	}

	// Orijinal boyuta kes (padding'i kaldır)
	if originalSize > 0 && originalSize < len(result) {
		result = result[:originalSize]
	}

	atomic.AddInt64(&e.stats.TotalDecoded, 1)
	return result, nil
}

// recoverBlock eksik bloğu XOR paritesinden kurtarır.
func (e *Encoder) recoverBlock(dataBlocks []Block, parity *Block, expectedCount int) *Block {
	recovered := make([]byte, e.blockSize)
	copy(recovered, parity.Data)

	for _, b := range dataBlocks {
		xorBytes(recovered, b.Data)
	}

	// Kurtarılan bloğun index'ini bul
	present := make(map[int]bool)
	for _, b := range dataBlocks {
		present[b.Index] = true
	}

	// Eksik index'i bul
	missingIdx := -1
	groupStart := parity.GroupID * (e.dataShards + 1)
	for i := groupStart; i < groupStart+expectedCount; i++ {
		if !present[i] {
			missingIdx = i
			break
		}
	}

	return &Block{
		Index:    missingIdx,
		IsParity: false,
		Data:     recovered,
		GroupID:  parity.GroupID,
	}
}

// Stats istatistikleri döndürür.
func (e *Encoder) Stats() EncoderStats {
	return EncoderStats{
		TotalEncoded:   atomic.LoadInt64(&e.stats.TotalEncoded),
		TotalDecoded:   atomic.LoadInt64(&e.stats.TotalDecoded),
		TotalRecovered: atomic.LoadInt64(&e.stats.TotalRecovered),
		TotalErrors:    atomic.LoadInt64(&e.stats.TotalErrors),
		BlocksCreated:  atomic.LoadInt64(&e.stats.BlocksCreated),
		ParityCreated:  atomic.LoadInt64(&e.stats.ParityCreated),
	}
}

// DataShards ayarını döndürür.
func (e *Encoder) DataShards() int { return e.dataShards }

// BlockSize ayarını döndürür.
func (e *Encoder) BlockSize() int { return e.blockSize }

// ── Yardımcı fonksiyonlar ──

func xorBytes(dst, src []byte) {
	n := len(dst)
	if len(src) < n {
		n = len(src)
	}
	for i := 0; i < n; i++ {
		dst[i] ^= src[i]
	}
}

func maxGroupID(blocks []Block) int {
	max := 0
	for _, b := range blocks {
		if b.GroupID > max {
			max = b.GroupID
		}
	}
	return max
}

func splitGroup(blocks []Block) (data []Block, parity *Block) {
	for i := range blocks {
		if blocks[i].IsParity {
			parity = &blocks[i]
		} else {
			data = append(data, blocks[i])
		}
	}
	return
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
