package binding

import (
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/TBXark/sphere/cmd/protoc-gen-sphere-binding/generate/log"
	"github.com/TBXark/sphere/proto/binding/sphere/binding"
	"github.com/fatih/structtag"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func GenerateFile(gen *protogen.Plugin, file *protogen.File, out string) {
	err := generateFile(gen, file, out)
	if err != nil {
		log.Warn("failed to generate file %s: %v", file.GeneratedFilenamePrefix, err)
	}
}

func generateFile(gen *protogen.Plugin, file *protogen.File, out string) error {
	tags := Extract(file)
	if len(tags) == 0 {
		return nil
	}

	filename := filepath.Join(out, file.GeneratedFilenamePrefix+".pb.go")

	fs := token.NewFileSet()
	fn, err := parser.ParseFile(fs, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	err = Retag(fn, tags)
	if err != nil {
		return err
	}

	var buf strings.Builder
	err = printer.Fprint(&buf, fs, fn)
	if err != nil {
		return err
	}

	source, err := format.Source([]byte(buf.String()))
	if err != nil {
		return err
	}

	err = os.WriteFile(filename, source, 0o644)
	if err != nil {
		return err
	}
	return nil
}

func Extract(file *protogen.File) StructTags {
	tags := make(StructTags)
	for _, message := range file.Messages {
		defaultBindingLocation := binding.BindingLocation_BINDING_LOCATION_UNSPECIFIED
		if proto.HasExtension(message.Desc.Options(), binding.E_DefaultLocation) {
			defaultBindingLocation = proto.GetExtension(message.Desc.Options(), binding.E_DefaultLocation).(binding.BindingLocation)
		}
		messageTags := make(map[string]*structtag.Tags)
		for _, field := range message.Fields {
			bindingLocation := defaultBindingLocation
			if proto.HasExtension(field.Desc.Options(), binding.E_Location) {
				bindingLocation = proto.GetExtension(field.Desc.Options(), binding.E_Location).(binding.BindingLocation)
			}
			fieldTags := &structtag.Tags{}
			switch bindingLocation {
			case binding.BindingLocation_BINDING_LOCATION_QUERY:
				_ = fieldTags.Set(&structtag.Tag{
					Key:     "form",
					Name:    string(field.Desc.Name()),
					Options: nil,
				})
				_ = fieldTags.Set(&structtag.Tag{
					Key:     "json",
					Name:    "-",
					Options: nil,
				})
			case binding.BindingLocation_BINDING_LOCATION_URI:
				_ = fieldTags.Set(&structtag.Tag{
					Key:     "uri",
					Name:    string(field.Desc.Name()),
					Options: nil,
				})
				_ = fieldTags.Set(&structtag.Tag{
					Key:     "json",
					Name:    "-",
					Options: nil,
				})
			}
			if fieldTags.Len() > 0 {
				messageTags[field.GoName] = fieldTags
			}
		}
		if len(messageTags) > 0 {
			tags[message.GoIdent.GoName] = messageTags
		}
	}
	return tags
}
