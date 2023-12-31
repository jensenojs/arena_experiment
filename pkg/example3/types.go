// copyright 2021 - 2023 matrix origin
//
// licensed under the apache license, version 2.0 (the "license");
// you may not use this file except in compliance with the license.
// you may obtain a copy of the license at
//
//      http://www.apache.org/licenses/license-2.0
//
// unless required by applicable law or agreed to in writing, software
// distributed under the license is distributed on an "as is" basis,
// without warranties or conditions of any kind, either express or implied.
// see the license for the specific language governing permissions and
// limitations under the license.

package example3

import (
	"arena_experiment/pkg/buffer"
)

type Identifier string

type IdentifierList []Identifier

type AliasClause struct {
	Cols *BufIdentifierList
}

type BufIdentifierList struct {
	l *IdentifierList
}

func NewBufIdentifierList(l IdentifierList) *BufIdentifierList {
	return &BufIdentifierList{&l}
}

func NewAliasClause(cs IdentifierList, buf *buffer.Buffer) *AliasClause {
	a := buffer.Alloc[AliasClause](buf)
	bcs := NewBufIdentifierList(cs)
	buf.Pin(bcs)
	a.Cols = bcs
	return a
}

type AliasClause2 struct {
	Cols IdentifierList
}

func NewAliasClause2(cs IdentifierList, buf *buffer.Buffer) *AliasClause2 {
	a := buffer.Alloc[AliasClause2](buf)
	a.Cols = cs
	return a
}
