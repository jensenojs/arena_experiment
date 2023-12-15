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

package example1

import (
	"arena_experiment/pkg/buffer"
	"testing"
)

func Example() {
	buf := buffer.New()
	defer buf.Free()

	p1 := NewPerson(1, buf)
	p2 := NewPerson(2, buf)

	p1.Friend = p2
}

func TestPointer(t *testing.T) {
	buf := buffer.New()
	defer buf.Free()

	p1 := NewPerson(1, buf)
	p2 := &Person{
		Id: 2,
	}

	// fatal error: unpinned Go pointer stored into non-Go memory !
	p1.Friend = p2
}
