// Copyright 2014-2015 The DevMine Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package src

type TypeDef struct {
	Name string   `json:"name"`
	Doc  string   `json:"doc"`
	Type ExprType `json:"type"`
}
