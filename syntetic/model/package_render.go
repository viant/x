package model

import (
	"os"
	"path/filepath"
)

// RenderFiles returns a map of filename -> source using each GoFile's Render().
func (p *Package) RenderFiles() (map[string]string, error) {
	out := map[string]string{}
	for _, gf := range p.Files {
		if gf == nil {
			continue
		}
		src, err := gf.Render()
		if err != nil {
			return nil, err
		}
		out[gf.Name] = src
	}
	return out, nil
}

// RenderFilesWithMethods renders each file using Render() (which includes method stubs)
// and appends free-standing function stubs per file.
func (p *Package) RenderFilesWithMethods() (map[string]string, error) {
	out := map[string]string{}
	funcsByFile := p.FunctionsByFile()
	for _, gf := range p.Files {
		if gf == nil {
			continue
		}
		src, err := gf.Render()
		if err != nil {
			return nil, err
		}
		// Append function stubs for this file.
		if fns := funcsByFile[gf.Name]; len(fns) > 0 {
			if len(src) > 0 && src[len(src)-1] != '\n' {
				src += "\n"
			}
			wrote := false
			for _, fn := range fns {
				if fn == nil || fn.Decl == nil {
					continue
				}
				if !wrote {
					src += "\n"
					wrote = true
				}
				stub := RenderFunctionStub(fn.Decl)
				if stub != "" {
					src += stub
					if src[len(src)-1] != '\n' {
						src += "\n"
					}
				}
			}
		}
		out[gf.Name] = src
	}
	return out, nil
}

// RenderFilesWithOptions renders files using per-file RenderWithOptions and
// optionally appends free-standing function stubs.
func (p *Package) RenderFilesWithOptions(opts RenderOptions, includeFreeFunctions bool) (map[string]string, error) {
	out := map[string]string{}
	funcsByFile := map[string][]*Function{}
	if includeFreeFunctions {
		funcsByFile = p.FunctionsByFile()
	}
	for _, gf := range p.Files {
		if gf == nil {
			continue
		}
		src, err := gf.RenderWithOptions(opts)
		if err != nil {
			return nil, err
		}
		if includeFreeFunctions {
			if fns := funcsByFile[gf.Name]; len(fns) > 0 {
				if len(src) > 0 && src[len(src)-1] != '\n' {
					src += "\n"
				}
				wrote := false
				for _, fn := range fns {
					if fn == nil || fn.Decl == nil {
						continue
					}
					if !wrote {
						src += "\n"
						wrote = true
					}
					stub := RenderFunctionStub(fn.Decl)
					if stub != "" {
						src += stub
						if src[len(src)-1] != '\n' {
							src += "\n"
						}
					}
				}
			}
		}
		out[gf.Name] = src
	}
	return out, nil
}

// PackageRenderOptions controls package-level rendering across all files.
// FileOptions, when provided, overrides the global Render options for
// specific filenames.
type PackageRenderOptions struct {
	Render               RenderOptions
	IncludeFreeFunctions bool
	FileOptions          map[string]RenderOptions // filename -> options
}

// RenderFilesWithPackageOptions renders all files using the provided package
// options. File-specific overrides take precedence over the global Render
// options. When IncludeFreeFunctions is true, free-standing functions are
// appended per file.
func (p *Package) RenderFilesWithPackageOptions(po PackageRenderOptions) (map[string]string, error) {
	out := map[string]string{}
	funcsByFile := map[string][]*Function{}
	if po.IncludeFreeFunctions {
		funcsByFile = p.FunctionsByFile()
	}
	for _, gf := range p.Files {
		if gf == nil {
			continue
		}
		opts := po.Render
		if po.FileOptions != nil {
			if o, ok := po.FileOptions[gf.Name]; ok {
				opts = o
			}
		}
		src, err := gf.RenderWithOptions(opts)
		if err != nil {
			return nil, err
		}
		if po.IncludeFreeFunctions {
			if fns := funcsByFile[gf.Name]; len(fns) > 0 {
				if len(src) > 0 && src[len(src)-1] != '\n' {
					src += "\n"
				}
				wrote := false
				for _, fn := range fns {
					if fn == nil || fn.Decl == nil {
						continue
					}
					if !wrote {
						src += "\n"
						wrote = true
					}
					stub := RenderFunctionStub(fn.Decl)
					if stub != "" {
						src += stub
						if src[len(src)-1] != '\n' {
							src += "\n"
						}
					}
				}
			}
		}
		out[gf.Name] = src
	}
	return out, nil
}

// WriteFiles writes rendered files to the specified directory using the default
// rendering (types with generics + method stubs). Directories are created as needed.
func (p *Package) WriteFiles(dir string) error {
	return p.WriteFilesWithOptions(dir, RenderOptions{}, false)
}

// WriteFilesWithMethods writes rendered files (including method stubs and free
// function stubs) to the specified directory.
func (p *Package) WriteFilesWithMethods(dir string) error {
	return p.WriteFilesWithOptions(dir, RenderOptions{}, true)
}

// WriteFilesWithOptions writes files rendered with the given options and flag
// controlling whether free functions are included.
func (p *Package) WriteFilesWithOptions(dir string, opts RenderOptions, includeFreeFunctions bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files, err := p.RenderFilesWithOptions(opts, includeFreeFunctions)
	if err != nil {
		return err
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// WriteFilesWithPackageOptions writes files using package-wide rendering
// options, including file-specific overrides.
func (p *Package) WriteFilesWithPackageOptions(dir string, po PackageRenderOptions) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files, err := p.RenderFilesWithPackageOptions(po)
	if err != nil {
		return err
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			return err
		}
	}
	return nil
}
