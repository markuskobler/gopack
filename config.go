package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml"
)

type Config struct {
	Checksum []byte
	// Path to the configuration file.
	Path string
	// Name of your repository "github.com/d2fn/gopack" for instance.
	Repository string
	// Dependencies tree
	DepsTree *toml.TomlTree
	// Development Dependencies tree
	DevDepsTree *toml.TomlTree
}

func NewConfig(dir string) *Config {
	config := &Config{Path: filepath.Join(dir, "gopack.config")}

	t, err := toml.LoadFile(config.Path)
	if err != nil {
		fail(err)
	}

	if deps := t.Get("deps"); deps != nil {
		config.DepsTree = deps.(*toml.TomlTree)
	}

	if deps := t.Get("dev-deps"); deps != nil {
		config.DevDepsTree = deps.(*toml.TomlTree)
	}

	if repo := t.Get("repo"); repo != nil {
		config.Repository = repo.(string)
	}

	return config
}

func (c *Config) InitRepo(importGraph *Graph) {
	if c.Repository != "" {
		src := filepath.Join(pwd, VendorDir, "src")
		os.MkdirAll(src, 0755)

		dir := filepath.Dir(c.Repository)
		base := filepath.Join(src, dir)
		os.MkdirAll(base, 0755)

		repo := filepath.Join(src, c.Repository)
		err := os.Symlink(pwd, repo)
		if err != nil && !os.IsExist(err) {
			fail(err)
		}

		dependency := NewDependency(c.Repository)
		importGraph.Insert(dependency)
	}
}

func (c *Config) modifiedChecksum() bool {
	dat, err := ioutil.ReadFile(c.checksumPath())
	return (err != nil && os.IsNotExist(err)) || !bytes.Equal(dat, c.checksum())
}

func (c *Config) WriteChecksum() {
	os.MkdirAll(filepath.Join(pwd, GopackDir), 0755)
	err := ioutil.WriteFile(c.checksumPath(), c.checksum(), 0644)

	if err != nil {
		fail(err)
	}
}

func (c *Config) checksumPath() string {
	return filepath.Join(pwd, GopackChecksum)
}

func (c *Config) checksum() []byte {
	if c.Checksum == nil {
		dat, err := ioutil.ReadFile(c.Path)
		if err != nil {
			fail(err)
		}

		h := md5.New()
		h.Write(dat)
		c.Checksum = h.Sum(nil)
	}
	return []byte(hex.EncodeToString(c.Checksum))
}

func (c *Config) LoadDependencyModel(importGraph *Graph) (deps *Dependencies, err error) {
	totalDeps := 0

	if c.DepsTree != nil {
		totalDeps += len(c.DepsTree.Keys())
	}
	if c.DevDepsTree != nil {
		totalDeps += len(c.DevDepsTree.Keys())
	}

	if totalDeps == 0 {
		return
	}

	deps = new(Dependencies)
	deps.Imports = make([]string, totalDeps)
	deps.Keys = make([]string, totalDeps)
	deps.DepList = make([]*Dep, totalDeps)
	deps.ImportGraph = importGraph

	modifiedChecksum := c.modifiedChecksum()

	if err := addDepsTree(deps, c.DepsTree, modifiedChecksum, 0); err != nil {
		return nil, err
	}
	if err := addDepsTree(deps, c.DevDepsTree, modifiedChecksum, len(c.DepsTree.Keys())); err != nil {
		return nil, err
	}
	return deps, nil
}

func addDepsTree(deps *Dependencies, depsTree *toml.TomlTree, modifiedChecksum bool, pos int) error {
	if depsTree == nil {
		return nil
	}
	for _, k := range depsTree.Keys() {

		depTree := depsTree.Get(k).(*toml.TomlTree)
		d := NewDependency(depTree.Get("import").(string))

		d.setScm(depTree)
		d.setSource(depTree)

		d.setCheckout(depTree, "branch", BranchFlag)
		d.setCheckout(depTree, "commit", CommitFlag)
		d.setCheckout(depTree, "tag", TagFlag)

		if err := d.Validate(); err != nil {
			return err
		}

		d.Fetch(modifiedChecksum)

		deps.Keys[pos] = k
		deps.Imports[pos] = d.Import
		deps.DepList[pos] = d

		pos++

		deps.ImportGraph.Insert(d)
	}
	return nil
}
