// Copyright 2014-2015 The DevMine Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package src

type Trait struct {
	Name       string       `json:"name"`
	Attributes []*Attribute `json:"attributes"`
	Methods    []*Method    `json:"methods"`
	Traits     []*Trait     `json:"traits"`
}
