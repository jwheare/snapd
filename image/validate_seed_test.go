// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package image_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/snapcore/snapd/image"
	"github.com/snapcore/snapd/snap"
	"github.com/snapcore/snapd/snap/snaptest"
	"github.com/snapcore/snapd/snap/squashfs"
)

type validateSuite struct {
	imageSuite
}

var _ = Suite(&validateSuite{})

var coreYaml = `name: core
version: 1.0
type: os`

var snapdYaml = `name: snapd
version: 1.0
type: snapd`

func (s *validateSuite) SetUpTest(c *C) {
	s.imageSuite.SetUpTest(c)

	err := os.MkdirAll(filepath.Join(s.root, "snaps"), 0755)
	c.Assert(err, IsNil)
}

func (s *validateSuite) makeSnapInSeed(c *C, snapYaml string) {
	info := infoFromSnapYaml(c, snapYaml, snap.R(1))

	src := snaptest.MakeTestSnapWithFiles(c, snapYaml, nil)
	dst := filepath.Join(s.root, "snaps", fmt.Sprintf("%s_%s.snap", info.InstanceName(), info.Revision.String()))

	err := os.Rename(src, dst)
	c.Assert(err, IsNil)
}

func (s *validateSuite) makeSeedYaml(c *C, seedYaml string) string {
	tmpf := filepath.Join(s.root, "seed.yaml")
	err := ioutil.WriteFile(tmpf, []byte(seedYaml), 0644)
	c.Assert(err, IsNil)
	return tmpf
}

func (s *validateSuite) TestValidateSnapHappy(c *C) {
	s.makeSnapInSeed(c, coreYaml)
	s.makeSnapInSeed(c, `name: gtk-common-themes
version: 19.04`)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: core
   channel: stable
   file: core_1.snap
 - name: gtk-common-themes
   channel: stable/ubuntu-19.04
   file: gtk-common-themes_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, IsNil)
}

func (s *validateSuite) TestValidateSnapMissingBase(c *C) {
	s.makeSnapInSeed(c, `name: need-base
base: some-base
version: 1.0`)
	s.makeSnapInSeed(c, coreYaml)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: core
   file: core_1.snap
 - name: need-base
   file: need-base_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- cannot use snap "need-base": base "some-base" is missing`)
}

func (s *validateSuite) TestValidateSnapMissingDefaultProvider(c *C) {
	s.makeSnapInSeed(c, coreYaml)
	s.makeSnapInSeed(c, `name: need-df
version: 1.0
plugs:
 gtk-3-themes:
  interface: content
  default-provider: gtk-common-themes
`)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: core
   file: core_1.snap
 - name: need-df
   file: need-df_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- cannot use snap "need-df": default provider "gtk-common-themes" is missing`)
}

func (s *validateSuite) TestValidateSnapSnapdHappy(c *C) {
	s.makeSnapInSeed(c, snapdYaml)
	s.makeSnapInSeed(c, packageCore18)
	s.makeSnapInSeed(c, `name: some-snap
version: 1.0
base: core18
`)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: snapd
   file: snapd_1.snap
 - name: some-snap
   file: some-snap_1.snap
 - name: core18
   file: core18_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, IsNil)
}

func (s *validateSuite) TestValidateSnapMissingCore(c *C) {
	s.makeSnapInSeed(c, snapdYaml)
	s.makeSnapInSeed(c, `name: some-snap
version: 1.0`)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: snapd
   file: snapd_1.snap
 - name: some-snap
   file: some-snap_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- cannot use snap "some-snap": required snap "core" missing`)
}

func (s *validateSuite) TestValidateSnapMissingSnapdAndCore(c *C) {
	s.makeSnapInSeed(c, packageCore18)
	s.makeSnapInSeed(c, `name: some-snap
version: 1.0
base: core18`)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: some-snap
   file: some-snap_1.snap
 - name: core18
   file: core18_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- the core or snapd snap must be part of the seed`)
}

func (s *validateSuite) TestValidateSnapMultipleErrors(c *C) {
	s.makeSnapInSeed(c, `name: some-snap
version: 1.0`)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: some-snap
   file: some-snap_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- the core or snapd snap must be part of the seed
- cannot use snap "some-snap": required snap "core" missing`)
}

func (s *validateSuite) TestValidateSnapSnapMissing(c *C) {
	s.makeSnapInSeed(c, coreYaml)
	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: core
   file: core_1.snap
 - name: some-snap
   file: some-snap_1.snap
`)

	err := image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- cannot open snap: open /.*/snaps/some-snap_1.snap: no such file or directory`)
}

func (s *validateSuite) TestValidateSnapSnapInvalid(c *C) {
	s.makeSnapInSeed(c, coreYaml)

	// "version" is missing in this yaml
	snapBuildDir := c.MkDir()
	snapYaml := `name: some-snap-invalid-yaml`
	metaSnapYaml := filepath.Join(snapBuildDir, "meta", "snap.yaml")
	err := os.MkdirAll(filepath.Dir(metaSnapYaml), 0755)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(metaSnapYaml, []byte(snapYaml), 0644)
	c.Assert(err, IsNil)

	// need to build the snap "manually" pack.Snap() will do validation
	snapFilePath := filepath.Join(c.MkDir(), "some-snap-invalid-yaml_1.snap")
	d := squashfs.New(snapFilePath)
	err = d.Build(snapBuildDir, "app")
	c.Assert(err, IsNil)

	// put the broken snap in place
	dst := filepath.Join(s.root, "snaps", "some-snap-invalid-yaml_1.snap")
	err = os.Rename(snapFilePath, dst)
	c.Assert(err, IsNil)

	seedFn := s.makeSeedYaml(c, `
snaps:
 - name: core
   file: core_1.snap
 - name: some-snap-invalid-yaml
   file: some-snap-invalid-yaml_1.snap
`)

	err = image.ValidateSeed(seedFn)
	c.Assert(err, ErrorMatches, `cannot validate seed:
- cannot use snap /.*/snaps/some-snap-invalid-yaml_1.snap: invalid snap version: cannot be empty`)
}
