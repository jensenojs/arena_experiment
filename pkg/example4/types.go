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

package example4

import (
	"arena_experiment/pkg/buffer"
)

type Person struct {
	name string
}

func NewPerson(name string, buf *buffer.Buffer) *Person {
	p := buffer.Alloc[Person](buf)
	p.name = name
	return p
}
