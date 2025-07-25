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
	tags, err := extractFile(file)
	if err != nil {
		return err
	}
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

func extractFile(file *protogen.File) (StructTags, error) {
	tags := make(StructTags)
	for _, message := range file.Messages {
		extraTags, err := extractMessage(message, binding.BindingLocation_BINDING_LOCATION_UNSPECIFIED, nil)
		if err != nil {
			return nil, err
		}
		for name, tag := range extraTags {
			if len(tag) > 0 {
				tags[name] = tag
			}
		}
	}
	return tags, nil
}

func extractMessage(message *protogen.Message, location binding.BindingLocation, autoTags []string) (StructTags, error) {
	tags := make(StructTags)

	if proto.HasExtension(message.Desc.Options(), binding.E_DefaultLocation) {
		location = proto.GetExtension(message.Desc.Options(), binding.E_DefaultLocation).(binding.BindingLocation)
	}
	if proto.HasExtension(message.Desc.Options(), binding.E_DefaultAutoTags) {
		autoTags = proto.GetExtension(message.Desc.Options(), binding.E_DefaultAutoTags).([]string)
	}

	messageTags := make(map[string]*structtag.Tags)
	// process fields
	for _, field := range message.Fields {
		fieldTags, err := extractField(field, location, autoTags)
		if err != nil {
			return nil, err
		}
		if fieldTags.Len() > 0 {
			messageTags[field.GoName] = fieldTags
		}
	}
	// process one_of
	for _, oneOf := range message.Oneofs {
		defaultOneOfBindingLocation := location
		defaultOneOfAutoTags := autoTags
		if proto.HasExtension(oneOf.Desc.Options(), binding.E_DefaultOneofLocation) {
			defaultOneOfBindingLocation = proto.GetExtension(oneOf.Desc.Options(), binding.E_DefaultOneofLocation).(binding.BindingLocation)
		}
		if proto.HasExtension(oneOf.Desc.Options(), binding.E_DefaultOneofAutoTags) {
			defaultOneOfAutoTags = proto.GetExtension(oneOf.Desc.Options(), binding.E_DefaultOneofAutoTags).([]string)
		}
		for _, field := range oneOf.Fields {
			fieldTags, err := extractField(field, defaultOneOfBindingLocation, defaultOneOfAutoTags)
			if err != nil {
				return nil, err
			}
			if fieldTags.Len() > 0 {
				messageTags[field.GoName] = fieldTags
			}
		}
	}
	// process nested messages
	for _, nested := range message.Messages {
		extraTags, err := extractMessage(nested, location, autoTags)
		if err != nil {
			return nil, err
		}
		for name, tag := range extraTags {
			tags[name] = tag
		}
	}

	tags[message.GoIdent.GoName] = messageTags
	return tags, nil
}

func extractField(field *protogen.Field, location binding.BindingLocation, autoTags []string) (*structtag.Tags, error) {
	if proto.HasExtension(field.Desc.Options(), binding.E_Location) {
		location = proto.GetExtension(field.Desc.Options(), binding.E_Location).(binding.BindingLocation)
	}
	if proto.HasExtension(field.Desc.Options(), binding.E_AutoTags) {
		autoTags = proto.GetExtension(field.Desc.Options(), binding.E_AutoTags).([]string)
	}
	fieldTags := &structtag.Tags{}

	// Add auto tags
	for _, tag := range autoTags {
		if tag == "" {
			continue
		}
		_ = fieldTags.Set(&structtag.Tag{
			Key:     tag,
			Name:    string(field.Desc.Name()),
			Options: nil,
		})
	}

	// Add sphere binding tags
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

	// Add manual tags
	if proto.HasExtension(field.Desc.Options(), binding.E_Tags) {
		tags := proto.GetExtension(field.Desc.Options(), binding.E_Tags).([]string)
		for _, tag := range tags {
			parse, err := structtag.Parse(tag)
			if err != nil {
				return nil, err
			}
			for _, t := range parse.Tags() {
				_ = fieldTags.Set(t)
			}
		}
	}
	return fieldTags, nil
}
