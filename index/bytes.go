package index

import (
	"github.com/balzaczyy/golucene/store"
)

type BytesStore struct {
	*store.DataOutput
	blocks    [][]byte
	blockSize uint32
	blockBits uint32
	blockMask uint32
	current   []byte
	nextWrite uint32
}

func newBytesStore(blockBits uint32) *BytesStore {
	blockSize := uint32(1) << blockBits
	self := &BytesStore{blockBits: blockBits,
		blockSize: blockSize,
		blockMask: blockSize - 1,
		nextWrite: blockSize}
	self.DataOutput = &store.DataOutput{
		WriteByte: func(b byte) error {
			if self.nextWrite == self.blockSize {
				self.current = make([]byte, self.blockSize)
				self.blocks = append(self.blocks, self.current)
				self.nextWrite = 0
			}
			self.current[self.nextWrite] = b
			self.nextWrite++
			return nil
		},
		WriteBytes: func(buf []byte) error {
			var offset uint32 = 0
			length := uint32(len(buf))
			for length > 0 {
				chunk := self.blockSize - self.nextWrite
				if length <= chunk {
					copy(self.current[self.nextWrite:], buf[offset:offset+length])
					self.nextWrite += length
					break
				} else {
					if chunk > 0 {
						copy(self.current[self.nextWrite:], buf[offset:offset+chunk])
						offset += chunk
						length -= chunk
					}
					self.current = make([]byte, self.blockSize)
					self.blocks = append(self.blocks, self.current)
					self.nextWrite = 0
				}
			}
			return nil
		}}
	return self
}

func (bs *BytesStore) forwardReader() *BytesReader {
	if len(bs.blocks) == 1 {
		return newForwardBytesReader(bs.blocks[0])
	}
	self := &ByteStoreForwardReader{nextRead: bs.blockSize}
	self.BytesReader = &BytesReader{
		skipBytes: func(count int32) {
			self.setPosition(self.getPosition() + int64(count))
		}, getPosition: func() int64 {
			return (int64(self.nextBuffer)-1)*int64(bs.blockSize) + int64(self.nextRead)
		}, setPosition: func(pos int64) {
			bufferIndex := pos >> bs.blockBits
			self.nextBuffer = uint32(bufferIndex + 1)
			self.current = bs.blocks[bufferIndex]
			self.nextRead = uint32(pos) & bs.blockMask
			// assert self.getPosition() == pos
		}, reversed: func() bool {
			return false
		}}
	self.DataInput = &store.DataInput{
		ReadByte: func() byte {
			if self.nextRead == bs.blockSize {
				self.current = bs.blocks[self.nextBuffer]
				self.nextBuffer++
				self.nextRead = 0
			}
			ans := self.current[self.nextRead]
			self.nextRead++
			return ans
		}, ReadBytes: func(buf []byte) (n int, err error) {
			var offset uint32 = 0
			length := uint32(len(buf))
			for length > 0 {
				chunkLeft := bs.blockSize - self.nextRead
				if length <= chunkLeft {
					copy(buf[offset:], self.current[self.nextRead:self.nextRead+length])
					self.nextRead += length
					break
				} else {
					if chunkLeft > 0 {
						copy(buf[offset:], self.current[self.nextRead:self.nextRead+chunkLeft])
						offset += chunkLeft
						length -= chunkLeft
					}
					self.current = bs.blocks[self.nextBuffer]
					self.nextBuffer++
					self.nextRead = 0
				}
			}
			return len(buf), nil
		}}
	return self.BytesReader
}

type ByteStoreForwardReader struct {
	*BytesReader
	current    []byte
	nextBuffer uint32
	nextRead   uint32
}

type ForwardBytesReader struct {
	*BytesReader
	bytes []byte
	pos   int32
}

func newForwardBytesReader(bytes []byte) *BytesReader {
	self := &ForwardBytesReader{}
	self.BytesReader = &BytesReader{
		skipBytes: func(count int32) {
			self.pos += count
		}, getPosition: func() int64 {
			return int64(self.pos)
		}, setPosition: func(pos int64) {
			self.pos = int32(pos)
		}, reversed: func() bool {
			return false
		}}
	self.DataInput = &store.DataInput{
		ReadByte: func() byte {
			self.pos++
			return self.bytes[self.pos-1]
		}, ReadBytes: func(buf []byte) (n int, err error) {
			copy(bytes[self.pos:], buf)
			self.pos += int32(len(buf))
			return len(buf), nil
		}}
	return self.BytesReader
}
