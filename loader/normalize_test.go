/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestNormalizeNetworkNames(t *testing.T) {
	wd, _ := os.Getwd()
	project := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
		Environment: map[string]string{
			"FOO": "BAR",
		},
		Networks: types.Networks{
			"myExternalnet": {
				Name:     "myExternalnet", // this is automaticaly setup by loader for externa networks before reaching normalization
				External: types.External{External: true},
			},
			"mynet": {},
			"myNamedNet": {
				Name: "CustomName",
			},
		},
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				Build: &types.BuildConfig{
					Context: "./testdata",
					Args: map[string]*string{
						"FOO": nil,
						"ZOT": nil,
					},
				},
				Scale: 1,
			},
		},
	}

	expected := `name: myProject
services:
  foo:
    build:
      context: ./testdata
      dockerfile: Dockerfile
      args:
        FOO: BAR
        ZOT: null
    networks:
      default: null
networks:
  default:
    name: myProject_default
  myExternalnet:
    name: myExternalnet
    external: true
  myNamedNet:
    name: CustomName
  mynet:
    name: myProject_mynet
`
	err := Normalize(&project, false)
	assert.NilError(t, err)
	marshal, err := project.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, expected, string(marshal))
}

func TestNormalizeResolvePathsBuildContextPaths(t *testing.T) {
	wd, _ := os.Getwd()
	project := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				Build: &types.BuildConfig{
					Context:    "./testdata",
					Dockerfile: "Dockerfile-sample",
				},
				Scale: 1,
			},
		},
	}

	expected := fmt.Sprintf(`name: myProject
services:
  foo:
    build:
      context: %s
      dockerfile: Dockerfile-sample
    networks:
      default: null
networks:
  default:
    name: myProject_default
`, filepath.Join(wd, "testdata"))
	err := Normalize(&project, true)
	assert.NilError(t, err)
	marshal, err := project.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, expected, string(marshal))
}

func TestNormalizeAbsolutePaths(t *testing.T) {
	project := types.Project{
		Name:         "myProject",
		WorkingDir:   "testdata",
		Networks:     types.Networks{},
		ComposeFiles: []string{filepath.Join("testdata", "simple", "compose.yaml"), filepath.Join("testdata", "simple", "compose-with-overrides.yaml")},
	}
	absWorkingDir, _ := filepath.Abs("testdata")
	absComposeFile, _ := filepath.Abs(filepath.Join("testdata", "simple", "compose.yaml"))
	absOverrideFile, _ := filepath.Abs(filepath.Join("testdata", "simple", "compose-with-overrides.yaml"))

	expected := types.Project{
		Name:         "myProject",
		Networks:     types.Networks{"default": {Name: "myProject_default"}},
		WorkingDir:   absWorkingDir,
		ComposeFiles: []string{absComposeFile, absOverrideFile},
	}
	err := Normalize(&project, false)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}

func TestNormalizeVolumes(t *testing.T) {
	project := types.Project{
		Name:     "myProject",
		Networks: types.Networks{},
		Volumes: types.Volumes{
			"myExternalVol": {
				Name:     "myExternalVol", // this is automaticaly setup by loader for externa networks before reaching normalization
				External: types.External{External: true},
			},
			"myvol": {},
			"myNamedVol": {
				Name: "CustomName",
			},
		},
	}

	absCwd, _ := filepath.Abs(".")
	expected := types.Project{
		Name:     "myProject",
		Networks: types.Networks{"default": {Name: "myProject_default"}},
		Volumes: types.Volumes{
			"myExternalVol": {
				Name:     "myExternalVol",
				External: types.External{External: true},
			},
			"myvol": {Name: "myProject_myvol"},
			"myNamedVol": {
				Name: "CustomName",
			},
		},
		WorkingDir:   absCwd,
		ComposeFiles: []string{},
	}
	err := Normalize(&project, false)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}

func TestNormalizeDependsOn(t *testing.T) {
	project := types.Project{
		Name:     "myProject",
		Networks: types.Networks{},
		Volumes:  types.Volumes{},
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				DependsOn: map[string]types.ServiceDependency{
					"bar": { // explicit depends_on never should be overridden
						Condition: types.ServiceConditionHealthy,
						Restart:   false,
					},
				},
				NetworkMode: "service:zot",
			},
			{
				Name: "bar",
				VolumesFrom: []string{
					"zot",
					"container:xxx",
				},
			},
			{
				Name: "zot",
			},
		},
	}

	expected := `name: myProject
services:
  bar:
    depends_on:
      zot:
        condition: service_started
    networks:
      default: null
    volumes_from:
      - zot
      - container:xxx
  foo:
    depends_on:
      bar:
        condition: service_healthy
      zot:
        condition: service_started
        restart: true
    network_mode: service:zot
  zot:
    networks:
      default: null
networks:
  default:
    name: myProject_default
`
	err := Normalize(&project, true)
	assert.NilError(t, err)
	marshal, err := project.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, expected, string(marshal))
}
