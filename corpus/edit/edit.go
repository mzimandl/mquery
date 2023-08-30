// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package edit

import (
	"encoding/binary"
	"fmt"
	"math"
	"mquery/corpus"
	"mquery/mango"
	"os"
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/fs"
)

const (
	maxSplitChunkSize = 100000000
)

func splitCorpusExists(subcBaseDir, corpusPath string) (bool, error) {
	cname := filepath.Base(corpusPath)
	path := filepath.Join(subcBaseDir, cname)
	isDir, err := fs.IsDir(path)
	if err != nil {
		return false, fmt.Errorf("failed to determine split corpus existence: %w", err)
	}
	if !isDir {
		return false, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("failed to determine split corpus existence: %w", err)
	}
	return isDir && len(entries) > 0, nil
}

func splitCorpus(subcBaseDir, corpusPath string) (corpus.SplitCorpus, error) {
	ans := corpus.SplitCorpus{CorpusPath: corpusPath}
	size, err := mango.GetCorpusSize(corpusPath)
	if err != nil {
		return ans, fmt.Errorf("failed create split corpus: %w", err)
	}
	numChunks := int(math.Ceil(float64(size) / float64(maxSplitChunkSize)))
	ans.Subcorpora = make([]string, numChunks)
	ans.CorpusPath = corpusPath
	cname := filepath.Base(corpusPath)
	corpDir := filepath.Join(subcBaseDir, cname)
	cdirExists, err := fs.IsDir(corpDir)
	if err != nil {
		return ans, fmt.Errorf("failed create split corpus: %w", err)
	}
	if !cdirExists {
		os.Mkdir(corpDir, 0755)
	}

	for i := 0; i < numChunks; i++ {
		path := filepath.Join(subcBaseDir, cname, fmt.Sprintf("chunk_%02d.subc", i))
		err := createSubcorpus(path, int64(i)*maxSplitChunkSize, int64(i+1)*maxSplitChunkSize)
		if err != nil {
			return ans, fmt.Errorf("failed to create split corpus: %w", err)
		}
		ans.Subcorpora[i] = path
	}
	return ans, nil
}

func createSubcorpus(path string, fromIdx int64, toIdx int64) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	bytesBuffer := make([]byte, 8, 8*2)
	binary.LittleEndian.PutUint64(bytesBuffer, uint64(fromIdx))
	bytesBuffer = binary.LittleEndian.AppendUint64(bytesBuffer, uint64(toIdx))
	_, err = file.Write(bytesBuffer)
	return err
}
