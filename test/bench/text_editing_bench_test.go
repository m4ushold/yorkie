//go:build bench

/*
 * Copyright 2021 The Yorkie Authors. All rights reserved.
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
	gojson "encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yorkie-team/yorkie/pkg/document"
	"github.com/yorkie-team/yorkie/pkg/document/json"
	"github.com/yorkie-team/yorkie/pkg/document/presence"
)

type EditOperation struct {
	Cursor        int
	DelCount      int
	InsertContent string
}

type TraceData struct {
	StartContent string
	EndContent   string
	Ops          []EditOperation
}

type rawTrace struct {
	StartContent string `json:"startContent"`
	EndContent   string `json:"endContent"`
	Txns         []struct {
		Patches [][]interface{} `json:"patches"`
	} `json:"txns"`
}

// readEditingTraceFromFile parses a trace JSON file into a usable structure.
func readEditingTraceFromFile(b *testing.B, path string) *TraceData {
	file, err := os.Open(path) // #nosec G304: opening known benchmark trace file (trusted input)
	if err != nil {
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		b.Fatal(err)
	}

	var raw rawTrace
	if err := gojson.Unmarshal(data, &raw); err != nil {
		b.Fatal(err)
	}

	var editOps []EditOperation
	for _, txn := range raw.Txns {
		for _, rawPatch := range txn.Patches {
			if len(rawPatch) != 3 {
				b.Fatalf("invalid patch format: %v", rawPatch)
			}
			editOps = append(editOps, EditOperation{
				Cursor:        int(rawPatch[0].(float64)),
				DelCount:      int(rawPatch[1].(float64)),
				InsertContent: rawPatch[2].(string),
			})
		}
	}

	return &TraceData{
		StartContent: raw.StartContent,
		EndContent:   raw.EndContent,
		Ops:          editOps,
	}
}

// replayEditingTrace performs all edit operations in order and asserts the result.
func replayEditingTrace(b *testing.B, trace *TraceData) {
	doc := document.New("d1")

	err := doc.Update(func(root *json.Object, p *presence.Presence) error {
		root.SetNewText("text").Edit(0, 0, trace.StartContent)
		return nil
	})
	assert.NoError(b, err)

	for _, op := range trace.Ops {
		err := doc.Update(func(root *json.Object, p *presence.Presence) error {
			text := root.GetText("text")
			text.Edit(op.Cursor, op.Cursor+op.DelCount, op.InsertContent)
			return nil
		})
		assert.NoError(b, err)
	}

	finalContent := doc.Root().GetText("text").String()
	assert.Equal(b, trace.EndContent, finalContent)
}

// BenchmarkTextEditing benchmarks all trace files sequentially.
func BenchmarkTextEditing(b *testing.B) {
	traceFiles := []string{
		"automerge-paper.json",
		"clownschool_flat.json",
		"friendsforever_flat.json",
		"json-crdt-blog-post.json",
		"json-crdt-patch.json",
		"rustcode.json",
		"seph-blog1.json",
		"sveltecomponent.json",
	}

	for _, file := range traceFiles {
		b.Run(file, func(b *testing.B) {
			trace := readEditingTraceFromFile(b, "editing-traces/"+file)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				replayEditingTrace(b, trace)
			}
		})
	}
}
