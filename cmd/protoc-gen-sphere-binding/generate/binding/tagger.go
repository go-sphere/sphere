package binding

import (
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/TBXark/sphere/proto/binding/sphere/binding"
	"github.com/fatih/structtag"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
)

func GenerateFile(file *protogen.File, out string) error {
	err := generateFile(file, out)
	if err != nil {
		return err
	}
	return nil
}

func generateFile(file *protogen.File, out string) error {
	tags := extractFile(file)
	if len(tags) == 0 {
		return nil
	}

	filename := filepath.Join(out, file.GeneratedFilenamePrefix+".pb.go")

	fs := token.NewFileSet()
	fn, err := parser.ParseFile(fs, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	err = ReTags(fn, tags)
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

func extractFile(file *protogen.File) StructTags {
	tags := make(StructTags)
	for _, message := range file.Messages {
		for name, tag := range extractMessage(message) {
			if len(tag) > 0 {
				tags[name] = tag
			}
		}
	}
	return tags
}

func extractMessage(message *protogen.Message) StructTags {
	tags := make(StructTags)
	defaultBindingLocation := binding.BindingLocation_BINDING_LOCATION_UNSPECIFIED
	if proto.HasExtension(message.Desc.Options(), binding.E_DefaultLocation) {
		defaultBindingLocation = proto.GetExtension(message.Desc.Options(), binding.E_DefaultLocation).(binding.BindingLocation)
	}
	messageTags := make(map[string]*structtag.Tags)
	for _, field := range message.Fields {
		bindingLocation := defaultBindingLocation
		fieldTags := extractField(field, bindingLocation)
		if fieldTags.Len() > 0 {
			messageTags[field.GoName] = fieldTags
		}
	}
	for _, oneOf := range message.Oneofs {
		defaultOneOfBindingLocation := defaultBindingLocation
		if proto.HasExtension(oneOf.Desc.Options(), binding.E_DefaultOneofLocation) {
			defaultOneOfBindingLocation = proto.GetExtension(oneOf.Desc.Options(), binding.E_DefaultOneofLocation).(binding.BindingLocation)
		}
		for _, field := range oneOf.Fields {
			bindingLocation := defaultOneOfBindingLocation
			fieldTags := extractField(field, bindingLocation)
			if fieldTags.Len() > 0 {
				messageTags[field.GoName] = fieldTags
			}
		}
	}
	for _, nested := range message.Messages {
		for name, tag := range extractMessage(nested) {
			tags[name] = tag
		}
	}
	tags[message.GoIdent.GoName] = messageTags
	return tags
}

func extractField(field *protogen.Field, defaultLocation binding.BindingLocation) *structtag.Tags {
	location := defaultLocation
	if proto.HasExtension(field.Desc.Options(), binding.E_Location) {
		location = proto.GetExtension(field.Desc.Options(), binding.E_Location).(binding.BindingLocation)
	}
	fieldTags := &structtag.Tags{}
	switch location {
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
	return fieldTags
}
