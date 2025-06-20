// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
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

package mango

// #cgo LDFLAGS:  -lmanatee -L${SRCDIR} -Wl,-rpath='$ORIGIN'
// #include <stdlib.h>
// #include "mango.h"
import "C"

import (
	"errors"
	"fmt"
	"mquery/merror"
	"strconv"
	"strings"
	"unicode"
	"unsafe"

	"github.com/czcorpus/cnc-gokit/collections"
	"github.com/czcorpus/cnc-gokit/maths"
	"github.com/czcorpus/mquery-common/concordance"
)

const (
	MaxRecordsInternalLimit = 1000
)

var (
	ErrRowsRangeOutOfConc = errors.New("rows range is out of concordance size")
)

type GoVector struct {
	v C.MVector
}

type Freqs struct {
	Words      []string
	Freqs      []int64
	Norms      []int64
	ConcSize   int64
	CorpusSize int64
	SubcSize   int64
}

// ---

type GoConcordance struct {
	Lines        []string
	AlignedLines []string
	ConcSize     int
	CorpusSize   int
}

// --------------------------

type GoTokenContext struct {
	Text string
}

type GoConcSize struct {
	Value      int64
	ARF        float64
	CorpusSize int64
}

type GoCollItem struct {
	Word  string  `json:"word"`
	Score float64 `json:"score"`
	Freq  int64   `json:"freq"`
}

type GoColls struct {
	Colls      []*GoCollItem
	ConcSize   int64
	CorpusSize int64
	SubcSize   int64
}

func GetCorpusSize(corpusPath string) (int64, error) {
	ans := C.get_corpus_size(C.CString(corpusPath))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return 0, err
	}
	return int64(ans.value), nil
}

func GetConcSize(corpusPath, query string) (GoConcSize, error) {
	ans := C.concordance_size(C.CString(corpusPath), C.CString(query))
	var ret GoConcSize
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return ret, err
	}
	ret.CorpusSize = int64(ans.corpusSize)
	ret.Value = int64(ans.value)
	ret.ARF = float64(ans.arf)
	return ret, nil
}

func CompileSubcFreqs(corpusPath, subcPath, attr string) error {
	ans := C.compile_subc_freqs(C.CString(corpusPath), C.CString(subcPath), C.CString(attr))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return err
	}

	return nil
}

func GetConcordance(
	corpusPath, query string,
	attrs []string,
	structs []string,
	refs []string,
	fromLine, maxItems, maxContext int,
	viewContextStruct string,
) (GoConcordance, error) {
	if fromLine < 0 {
		panic("GetConcordance - invalid fromLine value")
	}
	if maxItems < 0 {
		panic("GetConcordance - invalid maxItems value")
	}
	if maxContext < 0 {
		panic("GetConcordance - invalid maxContext value")
	}
	if !collections.SliceContains(refs, "#") {
		refs = append([]string{"#"}, refs...)
	}
	ans := C.conc_examples(
		C.CString(corpusPath),
		C.CString(query),
		C.CString(strings.Join(attrs, ",")),
		C.CString(strings.Join(structs, ",")),
		C.CString(strings.Join(refs, ",")),
		C.CString(concordance.RefsEndMark),
		C.longlong(fromLine),
		C.longlong(maxItems),
		C.longlong(maxContext),
		C.CString(viewContextStruct))
	var ret GoConcordance
	ret.Lines = make([]string, 0, maxItems)
	ret.ConcSize = int(ans.concSize)
	ret.CorpusSize = int(ans.corpusSize)
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		if ans.errorCode == 1 {
			return ret, ErrRowsRangeOutOfConc
		}
		return ret, err

	} else {
		defer C.conc_examples_free(ans.value, C.int(ans.size))
		if ans.aligned != nil {
			defer C.conc_examples_free(ans.aligned, C.int(ans.size))
		}
	}
	tmp := (*[MaxRecordsInternalLimit]*C.char)(unsafe.Pointer(ans.value))
	for i := 0; i < int(ans.size); i++ {
		str := C.GoString(tmp[i])
		// we must test str len as our c++ wrapper may return it
		// e.g. in case our offset is higher than actual num of lines
		if len(str) > 0 {
			ret.Lines = append(ret.Lines, str)
		}
	}

	// Process aligned lines if available
	if ans.aligned != nil {
		ret.AlignedLines = make([]string, 0, maxItems)
		alignedTmp := (*[MaxRecordsInternalLimit]*C.char)(unsafe.Pointer(ans.aligned))
		for i := 0; i < int(ans.size); i++ {
			str := C.GoString(alignedTmp[i])
			if len(str) > 0 {
				ret.AlignedLines = append(ret.AlignedLines, str)
			}
		}
	}

	return ret, nil
}

func GetConcordanceWithCollPhrase(
	corpusPath, query, collQuery string,
	lftCtx, rgtCtx int,
	attrs []string,
	structs []string,
	refs []string,
	fromLine, maxItems, maxContext int,
	viewContextStruct string,
) (GoConcordance, error) {
	if !collections.SliceContains(refs, "#") {
		refs = append([]string{"#"}, refs...)
	}
	ans := C.conc_examples_with_coll_phrase(
		C.CString(corpusPath),
		C.CString(query),
		C.CString(collQuery+";"),
		C.CString(strconv.Itoa(lftCtx)),
		C.CString(strconv.Itoa(rgtCtx)),
		C.CString(strings.Join(attrs, ",")),
		C.CString(strings.Join(structs, ",")),
		C.CString(strings.Join(refs, ",")),
		C.CString(concordance.RefsEndMark),
		C.longlong(fromLine),
		C.longlong(maxItems),
		C.longlong(maxContext),
		C.CString(viewContextStruct))
	var ret GoConcordance
	ret.Lines = make([]string, 0, maxItems)
	ret.ConcSize = int(ans.concSize)
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		if ans.errorCode == 1 {
			return ret, ErrRowsRangeOutOfConc
		}
		return ret, err

	} else {
		defer C.conc_examples_free(ans.value, C.int(ans.size))
		if ans.aligned != nil {
			defer C.conc_examples_free(ans.aligned, C.int(ans.size))
		}
	}
	tmp := (*[MaxRecordsInternalLimit]*C.char)(unsafe.Pointer(ans.value))
	for i := 0; i < int(ans.size); i++ {
		str := C.GoString(tmp[i])
		// we must test str len as our c++ wrapper may return it
		// e.g. in case our offset is higher than actual num of lines
		if len(str) > 0 {
			ret.Lines = append(ret.Lines, str)
		}
	}

	// Process aligned lines if available
	if ans.aligned != nil {
		ret.AlignedLines = make([]string, 0, maxItems)
		alignedTmp := (*[MaxRecordsInternalLimit]*C.char)(unsafe.Pointer(ans.aligned))
		for i := 0; i < int(ans.size); i++ {
			str := C.GoString(alignedTmp[i])
			if len(str) > 0 {
				ret.AlignedLines = append(ret.AlignedLines, str)
			}
		}
	}

	return ret, nil
}

func CalcFreqDist(corpusID, subcID, query, fcrit string, flimit int) (*Freqs, error) {
	var ret Freqs
	ans := C.freq_dist(C.CString(corpusID), C.CString(subcID), C.CString(query), C.CString(fcrit), C.longlong(flimit))
	defer func() { // the 'new' was called before any possible error so we have to do this
		C.delete_int_vector(ans.freqs)
		C.delete_int_vector(ans.norms)
		C.delete_str_vector(ans.words)
	}()
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return &ret, err
	}
	ret.Freqs = IntVectorToSlice(GoVector{ans.freqs})
	ret.Norms = IntVectorToSlice(GoVector{ans.norms})
	ret.Words = StrVectorToSlice(GoVector{ans.words})
	ret.ConcSize = int64(ans.concSize)
	ret.CorpusSize = int64(ans.corpusSize)
	ret.SubcSize = int64(ans.searchSize)
	return &ret, nil
}

func normalizeMultiword(w string) string {
	return strings.TrimSpace(strings.Map(func(c rune) rune {
		if unicode.IsSpace(c) {
			return ' '
		}
		return c
	}, w))
}

func StrVectorToSlice(vector GoVector) []string {
	size := int(C.str_vector_get_size(vector.v))
	slice := make([]string, size)
	for i := 0; i < size; i++ {
		cstr := C.str_vector_get_element(vector.v, C.int(i))
		slice[i] = normalizeMultiword(C.GoString(cstr))
	}
	return slice
}

func IntVectorToSlice(vector GoVector) []int64 {
	size := int(C.int_vector_get_size(vector.v))
	slice := make([]int64, size)
	for i := 0; i < size; i++ {
		v := C.int_vector_get_element(vector.v, C.int(i))
		slice[i] = int64(v)
	}
	return slice
}

// GetCollcations
//
// 't': 'T-score',
// 'm': 'MI',
// '3': 'MI3',
// 'l': 'log likelihood',
// 's': 'min. sensitivity',
// 'p': 'MI.log_f',
// 'r': 'relative freq. [%]',
// 'f': 'absolute freq.',
// 'd': 'logDice'
func GetCollcations(
	corpusID, subcID, query string,
	attrName string,
	measure byte,
	srchRange [2]int,
	minFreq int64,
	maxItems int,
) (GoColls, error) {
	colls := C.collocations(
		C.CString(corpusID), C.CString(subcID), C.CString(query), C.CString(attrName),
		C.char(measure), C.char(measure), C.longlong(minFreq), C.longlong(minFreq),
		C.int(srchRange[0]), C.int(srchRange[1]), C.int(maxItems))
	if colls.err != nil {
		err := fmt.Errorf(C.GoString(colls.err))
		defer C.free(unsafe.Pointer(colls.err))
		return GoColls{}, err
	}
	items := make([]*GoCollItem, colls.resultSize)
	for i := 0; i < int(colls.resultSize); i++ {
		tmp := C.get_coll_item(colls, C.int(i))
		items[i] = &GoCollItem{
			Word:  C.GoString(tmp.word),
			Score: maths.RoundToN(float64(tmp.score), 4),
			Freq:  int64(tmp.freq),
		}
	}
	//C.coll_examples_free(colls.items, colls.numItems)
	return GoColls{
		Colls:      items,
		ConcSize:   int64(colls.concSize),
		CorpusSize: int64(colls.corpusSize),
		SubcSize:   int64(colls.searchSize),
	}, nil
}

func GetTextTypesNorms(corpusPath string, attr string) (map[string]int64, error) {
	ans := make(map[string]int64)
	attrSplit := strings.Split(attr, ".")
	if len(attrSplit) != 2 {
		return ans,
			merror.InputError{
				Msg: fmt.Sprintf(
					"invalid attribute format in `%s` (must be `struct.attr`)",
					attr,
				),
			}
	}
	norms := C.get_attr_values_sizes(
		C.CString(corpusPath), C.CString(attrSplit[0]), C.CString(attrSplit[1]))
	if norms.err != nil {
		err := fmt.Errorf(C.GoString(norms.err))
		defer C.free(unsafe.Pointer(norms.err))
		return ans, err
	}
	defer C.delete_attr_values_sizes(norms.sizes)

	iter := C.get_attr_val_iterator(norms.sizes)
	defer C.delete_attr_val_iterator(iter)
	for {
		val := C.get_next_attr_val_size(norms.sizes, iter)
		if val.value == nil {
			break
		}
		ans[C.GoString(val.value)] = int64(val.freq)
	}

	return ans, nil
}

// GetCorpusConf returns a corpus configuration item
// stored in a corpus configuration file (aka "registry file")
func GetCorpusConf(corpusPath string, prop string) (string, error) {
	ans := (C.get_corpus_conf(C.open_corpus(C.CString(corpusPath)).value, C.CString(prop)))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return "", err
	}
	return C.GoString(ans.value), nil
}

func GetPosAttrSize(corpusPath string, name string) (int, error) {
	ans := C.get_posattr_size(C.CString(corpusPath), C.CString(name))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return 0, err
	}
	return int(ans.value), nil
}

func GetStructSize(corpusPath string, name string) (int, error) {
	ans := C.get_struct_size(C.CString(corpusPath), C.CString(name))
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return 0, err
	}
	return int(ans.value), nil
}

func GetCorpRegion(corpusPath string, lftCtx, rgtCtx int64, structs, attrs []string) (GoTokenContext, error) {
	ans := C.get_corp_region(
		C.CString(corpusPath),
		C.longlong(lftCtx),
		C.longlong(rgtCtx),
		C.CString(strings.Join(attrs, ",")),
		C.CString(strings.Join(structs, ",")),
	)
	if ans.err != nil {
		err := fmt.Errorf(C.GoString(ans.err))
		defer C.free(unsafe.Pointer(ans.err))
		return GoTokenContext{}, err
	}
	defer C.free_string(ans.text)
	return GoTokenContext{Text: C.GoString(ans.text)}, nil
}
