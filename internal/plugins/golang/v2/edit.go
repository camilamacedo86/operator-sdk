// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"github.com/spf13/pflag"

	"sigs.k8s.io/kubebuilder/pkg/model/config"
	"sigs.k8s.io/kubebuilder/pkg/plugin"
)

type editPlugin struct {
	plugin.Edit

	config *config.Config
}

var _ plugin.Edit = &editPlugin{}

func (p *editPlugin) UpdateContext(ctx *plugin.Context) { p.Edit.UpdateContext(ctx) }
func (p *editPlugin) BindFlags(fs *pflag.FlagSet)       { p.Edit.BindFlags(fs) }

func (p *editPlugin) InjectConfig(c *config.Config) {
	p.Edit.InjectConfig(c)
	p.config = c
}

func (p *editPlugin) Run() error {
	return p.Edit.Run()
}
