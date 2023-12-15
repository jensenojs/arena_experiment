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

package example2

import (
	"arena_experiment/pkg/buffer"
	"go/constant"
)

type Person struct {
	Id *BufConstant
}

func NewPerson(id *BufConstant, buf *buffer.Buffer) *Person {
	p := buffer.Alloc[Person](buf)
	p.Id = id
	return p
}

type Person2 struct {
	Id constant.Value // can not alloc by buffer!
}

func NewPerson2(id constant.Value, buf *buffer.Buffer) *Person2 {
	p := buffer.Alloc[Person2](buf)
	p.Id = id
	return p
}

type BufConstant struct {
	v *constant.Value
}

func NewBufConstant(value constant.Value) *BufConstant {
	return &BufConstant{&value}
}

func (b *BufConstant) Get() constant.Value {
	if b != nil && b.v != nil {
		return *b.v
	}
	return nil
}
