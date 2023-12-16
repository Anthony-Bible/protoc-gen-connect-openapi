package converter

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"github.com/swaggest/openapi-go/openapi31"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type Options struct {
	// Format can be either "yaml" or "json"
	Format  string
	Version string
}

func parseOptions(s string) (Options, error) {
	opts := Options{
		Version: "v1.0.0",
		Format:  "yaml",
	}

	for _, param := range strings.Split(s, ",") {
		switch param {
		case "":
		case "format_yaml":
			opts.Format = "yaml"
		case "format_json":
			opts.Format = "json"
		default:
			if strings.HasPrefix(param, "version=") {
				opts.Version = param[8:]
			} else {
				return opts, fmt.Errorf("invalid parameter: %s", param)
			}
		}
	}
	return opts, nil
}

func ConvertFrom(rd io.Reader) (*plugin.CodeGeneratorResponse, error) {
	input, err := io.ReadAll(rd)
	if err != nil {
		return nil, fmt.Errorf("failed to read request: %w", err)
	}

	req := &plugin.CodeGeneratorRequest{}
	err = proto.Unmarshal(input, req)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal input: %w", err)
	}

	return Convert(req)
}

func formatTypeRef(t string) string {
	return strings.TrimPrefix(t, ".")
}

func Convert(req *plugin.CodeGeneratorRequest) (*plugin.CodeGeneratorResponse, error) {
	opts, err := parseOptions(req.GetParameter())
	if err != nil {
		return nil, err
	}

	files := []*pluginpb.CodeGeneratorResponse_File{}
	genFiles := make(map[string]struct{}, len(req.FileToGenerate))
	for _, file := range req.FileToGenerate {
		genFiles[file] = struct{}{}
	}

	// We need this to resolve dependencies when making protodesc versions of the files
	resolver, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: req.GetProtoFile(),
	})
	if err != nil {
		return nil, err
	}

	for _, fileDesc := range req.GetProtoFile() {
		if _, ok := genFiles[fileDesc.GetName()]; !ok {
			slog.Debug("skip generating file because it wasn't requested", slog.String("name", fileDesc.GetName()))
			continue
		}

		slog.Info("generating file", slog.String("name", fileDesc.GetName()))

		fd, err := protodesc.NewFile(fileDesc, resolver)
		if err != nil {
			slog.Error("error loading file", slog.Any("error", err))
			return nil, err
		}

		spec := openapi31.Spec{Openapi: "3.1.0"}
		spec.SetTitle(string(fd.FullName()))
		spec.SetDescription(formatComments(fd.SourceLocations().ByDescriptor(fd)))
		spec.SetVersion(opts.Version)

		// Add all messages/enums as top-level types
		components, err := fileToComponents(fd)
		if err != nil {
			return nil, err
		}
		spec.WithComponents(components)

		pathItems, err := fileToPathItems(fd)
		if err != nil {
			return nil, err
		}
		spec.WithPaths(openapi31.Paths{MapOfPathItemValues: pathItems})
		spec.WithTags(fileToTags(fd)...)

		switch opts.Format {
		case "yaml":
			b, err := spec.MarshalYAML()
			if err != nil {
				return nil, err
			}

			content := string(b)
			name := fileDesc.GetName()
			filename := strings.TrimSuffix(name, filepath.Ext(name)) + ".openapi.yaml"

			files = append(files, &pluginpb.CodeGeneratorResponse_File{
				Name:              &filename,
				Content:           &content,
				GeneratedCodeInfo: &descriptorpb.GeneratedCodeInfo{},
			})
		case "json":
			b, err := json.MarshalIndent(spec, "", "  ")
			if err != nil {
				return nil, err
			}

			content := string(b)
			name := fileDesc.GetName()
			filename := strings.TrimSuffix(name, filepath.Ext(name)) + ".openapi.json"

			files = append(files, &pluginpb.CodeGeneratorResponse_File{
				Name:              &filename,
				Content:           &content,
				GeneratedCodeInfo: &descriptorpb.GeneratedCodeInfo{},
			})
		}
	}

	return &plugin.CodeGeneratorResponse{
		File: files,
	}, nil
}

func formatComments(loc protoreflect.SourceLocation) string {
	var builder strings.Builder
	if loc.LeadingComments != "" {
		builder.WriteString(strings.TrimSpace(loc.LeadingComments))
		builder.WriteString(" ")
	}
	if loc.TrailingComments != "" {
		builder.WriteString(strings.TrimSpace(loc.TrailingComments))
		builder.WriteString(" ")
	}
	return strings.TrimSpace(builder.String())
}
