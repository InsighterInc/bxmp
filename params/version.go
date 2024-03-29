// Copyright 2016 The BXMP Authors
// This file is part of the BXMP library.
//
// The BXMP library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The BXMP library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the BXMP library. If not, see <http://www.gnu.org/licenses/>.

package params

import (
	"fmt"
)

const (
	VersionMajor = 1        // Major version component of the current release
	VersionMinor = 7        // Minor version component of the current release
	VersionPatch = 2        // Patch version component of the current release
	VersionMeta  = "stable" // Version metadata to append to the version string

	BitmedVersionMajor = 0
	BitmedVersionMinor = 8
	BitmedVersionPatch = 0
)

// Version holds the textual version string.
var Version = func() string {
	v := fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)
	if VersionMeta != "" {
		v += "-" + VersionMeta
	}

	return v
}()

// Version holds the textual version string.
var BitmedVersion = func() string {
	return fmt.Sprintf("%d.%d.%d", BitmedVersionMajor, BitmedVersionMinor, BitmedVersionPatch)
}()

func VersionWithCommit(gitCommit string) string {
	vsn := Version
	if len(gitCommit) >= 8 {
		vsn += "-" + gitCommit[:8]
	}
	return vsn
}
