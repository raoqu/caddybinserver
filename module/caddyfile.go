// Copyright 2015 Matthew Holt and The Caddy Authors
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

package module

import (
	"io/fs"
	"path/filepath"
	"strings"

	caddy "github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/encode"
)

func init() {
	httpcaddyfile.RegisterHandlerDirective("bin_server", parseCaddyfile)
}

// parseCaddyfile parses the file_server directive. It enables the static file
// server and configures it with this syntax:
//
//	file_server [<matcher>] [browse] {
//	    fs            <backend...>
//	    root          <path>
//	    hide          <files...>
//	    index         <files...>
//	    browse        [<template_file>]
//	    precompressed <formats...>
//	    status        <status>
//	    disable_canonical_uris
//	}
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var fsrv BinServer

	for h.Next() {
		args := h.RemainingArgs()
		switch len(args) {
		case 0:
		case 1:
			if args[0] != "browse" {
				return nil, h.ArgErr()
			}
			fsrv.Browse = new(Browse)
		default:
			return nil, h.ArgErr()
		}

		for h.NextBlock(0) {
			switch h.Val() {
			case "fs":
				if !h.NextArg() {
					return nil, h.ArgErr()
				}
				if fsrv.FileSystemRaw != nil {
					return nil, h.Err("file system module already specified")
				}
				name := h.Val()
				modID := "caddy.fs." + name
				unm, err := caddyfile.UnmarshalModule(h.Dispenser, modID)
				if err != nil {
					return nil, err
				}
				fsys, ok := unm.(fs.FS)
				if !ok {
					return nil, h.Errf("module %s (%T) is not a supported file system implementation (requires fs.FS)", modID, unm)
				}
				fsrv.FileSystemRaw = caddyconfig.JSONModuleObject(fsys, "backend", name, nil)

			case "hide":
				fsrv.Hide = h.RemainingArgs()
				if len(fsrv.Hide) == 0 {
					return nil, h.ArgErr()
				}

			case "index":
				fsrv.IndexNames = h.RemainingArgs()
				if len(fsrv.IndexNames) == 0 {
					return nil, h.ArgErr()
				}

			case "root":
				if !h.Args(&fsrv.Root) {
					return nil, h.ArgErr()
				}

			case "browse":
				if fsrv.Browse != nil {
					return nil, h.Err("browsing is already configured")
				}
				fsrv.Browse = new(Browse)
				h.Args(&fsrv.Browse.TemplateFile)

			case "precompressed":
				var order []string
				for h.NextArg() {
					modID := "http.precompressed." + h.Val()
					mod, err := caddy.GetModule(modID)
					if err != nil {
						return nil, h.Errf("getting module named '%s': %v", modID, err)
					}
					inst := mod.New()
					precompress, ok := inst.(encode.Precompressed)
					if !ok {
						return nil, h.Errf("module %s is not a precompressor; is %T", modID, inst)
					}
					if fsrv.PrecompressedRaw == nil {
						fsrv.PrecompressedRaw = make(caddy.ModuleMap)
					}
					fsrv.PrecompressedRaw[h.Val()] = caddyconfig.JSON(precompress, nil)
					order = append(order, h.Val())
				}
				fsrv.PrecompressedOrder = order

			case "status":
				if !h.NextArg() {
					return nil, h.ArgErr()
				}
				fsrv.StatusCode = caddyhttp.WeakString(h.Val())

			case "disable_canonical_uris":
				if h.NextArg() {
					return nil, h.ArgErr()
				}
				falseBool := false
				fsrv.CanonicalURIs = &falseBool

			case "pass_thru":
				if h.NextArg() {
					return nil, h.ArgErr()
				}
				fsrv.PassThru = true

			default:
				return nil, h.Errf("unknown subdirective '%s'", h.Val())
			}
		}
	}

	// hide the Caddyfile (and any imported Caddyfiles)
	if configFiles := h.Caddyfiles(); len(configFiles) > 0 {
		for _, file := range configFiles {
			file = filepath.Clean(file)
			if !fileHidden(file, fsrv.Hide) {
				// if there's no path separator, the file server module will hide all
				// files by that name, rather than a specific one; but we want to hide
				// only this specific file, so ensure there's always a path separator
				if !strings.Contains(file, separator) {
					file = "." + separator + file
				}
				fsrv.Hide = append(fsrv.Hide, file)
			}
		}
	}

	return &fsrv, nil
}
