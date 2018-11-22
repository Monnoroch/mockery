package mockery

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Walker struct {
	BaseDir   string
	Recursive bool
	Filter    *regexp.Regexp
	LimitOne  bool
	BuildTags []string
}

type WalkerVisitor interface {
	VisitWalk(*Interface) error
}

func (this *Walker) Walk(visitor WalkerVisitor) (generated bool) {
	parser := NewParser(this.BuildTags)
	this.doWalk(parser, this.BaseDir, visitor)

	err := parser.Load()
	if err != nil {
		fmt.Fprintf(os.Stdout, "Error walking: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("DEBUG: Found interfaces %d\n", len(parser.Interfaces()))
	for _, iface := range parser.Interfaces() {
		fmt.Printf("DEBUG: Found interface %s\n", iface)

		if !this.Filter.MatchString(iface.Name) {
			continue
		}
		err := visitor.VisitWalk(iface)
		if err != nil {
			fmt.Fprintf(os.Stdout, "Error walking %s: %s\n", iface.Name, err)
			os.Exit(1)
		}

		fmt.Printf("DEBUG: Generated for interface %s\n", iface)
		generated = true
		if this.LimitOne {
			return
		}
	}

	return
}

func (this *Walker) doWalk(p *Parser, dir string, visitor WalkerVisitor) (generated bool) {
	fmt.Printf("DEBUG: Walk dir %s\n", dir)

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}


	for _, file := range files {
		fmt.Printf("DEBUG: Found file %s\n", file.Name())

		if strings.HasPrefix(file.Name(), ".") || strings.HasPrefix(file.Name(), "_") {
			continue
		}

		path := filepath.Join(dir, file.Name())

		if file.IsDir() {
			if this.Recursive {
				generated = this.doWalk(p, path, visitor) || generated
				if generated && this.LimitOne {
					return
				}
			}
			continue
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}

		fmt.Printf("DEBUG: Parse file %s\n", path)

		err = p.Parse(path)
		if err != nil {
			fmt.Printf("DEBUG: Parse error %s -- %v\n", path, err)
			fmt.Fprintln(os.Stdout, "Error parsing file: ", err)
			continue
		}
	}

	return
}

type GeneratorVisitor struct {
	InPackage bool
	Note      string
	Osp       OutputStreamProvider
	// The name of the output package, if InPackage is false (defaults to "mocks")
	PackageName string
}

func (this *GeneratorVisitor) VisitWalk(iface *Interface) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Unable to generated mock for '%s': %s\n", iface.Name, r)
			return
		}
	}()

	var out io.Writer
	var pkg string

	if this.InPackage {
		pkg = iface.Path
	} else {
		pkg = this.PackageName
	}

	out, err, closer := this.Osp.GetWriter(iface, pkg)
	if err != nil {
		fmt.Printf("Unable to get writer for %s: %s", iface.Name, err)
		os.Exit(1)
	}
	defer closer()

	gen := NewGenerator(iface, pkg, this.InPackage)
	gen.GeneratePrologueNote(this.Note)
	gen.GeneratePrologue(pkg)

	err = gen.Generate()
	if err != nil {
		return err
	}

	err = gen.Write(out)
	if err != nil {
		return err
	}
	return nil
}
