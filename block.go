/*
	Copyright (C) 2024-2025  Pancakes <patapancakes@pagefault.games>

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package gozelle

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Block struct {
	Offset uint64 `json:"offset"`
	Length uint64 `json:"length"`

	data   io.Reader
	closer io.Closer
}

var ErrBlockNotPrepared = errors.New("block not prepared")

func (b Block) Read(dst []byte) (int, error) {
	if b.Length == 0 {
		return 0, io.EOF
	}

	if b.data == nil {
		return 0, ErrBlockNotPrepared
	}

	n, err := b.data.Read(dst)
	if err != nil {
		return n, err
	}

	return n, nil
}

func (b Block) Close() error {
	if b.closer == nil {
		return nil
	}

	b.closer.Close()

	return nil
}

func (b *Block) Prepare(key []byte, src io.ReaderAt, mode Mode) error {
	// why do zero-length blocks exist?
	if b.Length == 0 {
		return nil
	}

	block := make([]byte, b.Length)
	_, err := src.ReadAt(block, int64(b.Offset))
	if err != nil {
		return fmt.Errorf("failed to read data: %s", err)
	}

	b.data = bytes.NewReader(block)

	// zlib buffer sizes if encrypted, not used
	var encSize, decSize uint32
	if mode == EncryptedCompressed {
		err = read(b.data, binary.LittleEndian, &encSize, &decSize)
		if err != nil {
			return fmt.Errorf("failed to read value: %s", err)
		}
	}

	// decrypt
	if mode == EncryptedCompressed || mode == Encrypted {
		if key == nil {
			return fmt.Errorf("missing decryption key")
		}

		block, err := aes.NewCipher(key)
		if err != nil {
			return fmt.Errorf("failed to create aes cipher: %s", err)
		}

		b.data = &cipher.StreamReader{S: cipher.NewCFBDecrypter(block, make([]byte, 0x10)), R: b.data}
	}

	// decompress
	if mode == EncryptedCompressed || mode == Compressed {
		zr, err := zlib.NewReader(b.data)
		if err != nil {
			return fmt.Errorf("failed to create zlib reader: %s", err)
		}

		b.closer = zr
		b.data = zr
	}

	return nil
}
