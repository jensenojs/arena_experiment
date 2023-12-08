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
	"testing"
)

// 无法通过buffer alloc来获得的对象
func Example2() {
// 	buf := buffer.New()
// 	defer buf.Free()

// 	// p1 := NewPerson(1, buf)
// 	// p2 := NewPerson(2, buf)

// 	p1.Friend = p2
}

func TestConstantValue(t *testing.T) {
	buf := buffer.New()
	defer buf.Free()

	id := constant.MakeInt64(1)
	_ = NewPerson(id, buf)
	// fatal error: unpinned Go pointer stored into non-Go memory !
}
