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

	"sigs.k8s.io/kubebuilder/v2/pkg/model/config"
	"sigs.k8s.io/kubebuilder/v2/pkg/plugin"
)

type editSubcommand struct {
	plugin.EditSubcommand

	config *config.Config
}

var _ plugin.EditSubcommand = &editSubcommand{}

func (p *editSubcommand) UpdateContext(ctx *plugin.Context) { p.UpdateContext(ctx) }
func (p *editSubcommand) BindFlags(fs *pflag.FlagSet)       { p.BindFlags(fs) }

func (p *editSubcommand) InjectConfig(c *config.Config) {
	p.InjectConfig(c)
	p.config = c
}

func (p *editSubcommand) Run() error {
	return p.Run()
}
