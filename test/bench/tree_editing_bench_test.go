//go:build bench

/*
 * Copyright 2024 The Yorkie Authors. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bench

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yorkie-team/yorkie/api/converter"
	"github.com/yorkie-team/yorkie/pkg/document/crdt"
	"github.com/yorkie-team/yorkie/pkg/document/json"
	"github.com/yorkie-team/yorkie/pkg/splay"
	"github.com/yorkie-team/yorkie/test/helper"
)

type stringValue struct {
	content string
	removed bool
}

func newSplayNode(content string) *splay.Node[*stringValue] {
	return splay.NewNode(&stringValue{
		content: content,
	})
}

func (v *stringValue) Len() int {
	if v.removed {
		return 0
	}
	return len(v.content)
}

func (v *stringValue) String() string {
	return v.content
}

func buildSplayTree(size int) *splay.Tree[*stringValue] {
	tree := splay.NewTree[*stringValue](nil)
	for i := 0; i < size; i++ {
		tree.Insert(newSplayNode("a"))
	}
	return tree
}

func BenchmarkSplayTree(b *testing.B) {
	operationCount := []int{10000, 20000, 30000}
	rand.Seed(time.Now().UnixNano())

	for _, size := range operationCount {
		b.ResetTimer()

		b.Run(fmt.Sprintf("Insert %d", size), func(b *testing.B) {
			tree := splay.NewTree[*stringValue](nil)
			for i := 0; i < size; i++ {
				tree.Insert(newSplayNode("a"))
			}
		})
		b.Run(fmt.Sprintf("Random read %d", size), func(b *testing.B) {
			b.StopTimer()
			tree := buildSplayTree(size)
			b.StartTimer()

			for i := 0; i < size; i++ {
				tree.Find(rand.Intn(size - i))
			}
		})
		b.Run(fmt.Sprintf("Delete %d", size), func(b *testing.B) {
			b.StopTimer()
			tree := buildSplayTree(size)
			b.StartTimer()

			for i := 0; i < size; i++ {
				node, _, _ := tree.Find(rand.Intn(size))
				if node != nil {
					tree.Delete(node)
				}
			}
		})
	}

	b.Run("edit splay tree", func(b *testing.B) {
		b.StopTimer()

		editingTrace, err := readEditingTraceFromFile(b)
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
		tree := splay.NewTree[*stringValue](nil)
		for _, edit := range editingTrace.Edits {
			cursor := int(edit[0].(float64))
			mode := int(edit[1].(float64))

			if mode == 0 {
				strValue, ok := edit[2].(string)
				if ok {
					tree.Insert(newSplayNode(strValue))
				}
			} else {
				node, _, err := tree.Find(cursor)
				if err != nil && node != nil {
					tree.Delete(node)
				}
			}
		}
	})
}

func BenchmarkTree(b *testing.B) {
	verticesCounts := []int{10000, 20000, 30000}

	for _, cnt := range verticesCounts {
		root := buildTree(cnt)
		b.ResetTimer()

		b.Run(fmt.Sprintf("%d vertices to protobuf", cnt), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = converter.ToTreeNodes(root)
			}
		})

		b.Run(fmt.Sprintf("%d vertices from protobuf", cnt), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				pbNodes := converter.ToTreeNodes(root)
				_, err := converter.FromTreeNodes(pbNodes)
				assert.NoError(b, err)
			}
		})
	}
}

// buildTree creates a tree with the given number of vertices.
func buildTree(vertexCnt int) *crdt.TreeNode {
	children := make([]json.TreeNode, vertexCnt)
	for i := 0; i < vertexCnt; i++ {
		children[i] = json.TreeNode{
			Type: "p", Children: []json.TreeNode{{Type: "text", Value: "a"}},
		}
	}

	return helper.BuildTreeNode(&json.TreeNode{
		Type:     "r",
		Children: children,
	})
}
