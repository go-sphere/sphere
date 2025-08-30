package bind

// Options controls the behavior of binding operations through various field-level configurations.
// It provides fine-grained control over how fields are processed during data binding.
type Options struct {
	ignoreSetZeroFields map[string]struct{}
	clearOnNilFields    map[string]struct{}
	ignoreFields        map[string]struct{}
	keepFieldsOnly      map[string]struct{}
}

// NewBindOptions creates a new Options instance with the provided option functions applied.
// This is the recommended way to create binding options with custom configurations.
func NewBindOptions(options ...Option) *Options {
	defaults := &Options{}
	for _, opt := range options {
		opt(defaults)
	}
	return defaults
}

// Option defines a function type for configuring Options instances.
// This pattern allows for flexible and composable option configuration.
type Option func(*Options)

// IgnoreSetZeroField creates an option that prevents setting zero values for specified fields.
// Fields marked with this option will not be set if they contain zero values.
func IgnoreSetZeroField(fields ...string) Option {
	return func(o *Options) {
		if o.ignoreSetZeroFields == nil {
			o.ignoreSetZeroFields = make(map[string]struct{}, len(fields))
		}
		for _, field := range fields {
			o.ignoreSetZeroFields[field] = struct{}{}
		}
	}
}

// ClearOnNilField creates an option that clears fields when nil values are encountered.
// This is useful for properly handling nullable fields in data binding operations.
func ClearOnNilField(fields ...string) Option {
	return func(o *Options) {
		if o.clearOnNilFields == nil {
			o.clearOnNilFields = make(map[string]struct{}, len(fields))
		}
		for _, field := range fields {
			o.clearOnNilFields[field] = struct{}{}
		}
	}
}

// IgnoreField creates an option that completely excludes specified fields from binding operations.
// Fields marked with this option will be skipped entirely during data binding.
func IgnoreField(fields ...string) Option {
	return func(o *Options) {
		if o.ignoreFields == nil {
			o.ignoreFields = make(map[string]struct{}, len(fields))
		}
		for _, field := range fields {
			o.ignoreFields[field] = struct{}{}
		}
	}
}

// KeepFieldsOnly creates an option that restricts binding to only the specified fields.
// When this option is used, all other fields will be ignored during binding operations.
func KeepFieldsOnly(fields ...string) Option {
	return func(o *Options) {
		if o.keepFieldsOnly == nil {
			o.keepFieldsOnly = make(map[string]struct{}, len(fields))
		}
		for _, field := range fields {
			o.keepFieldsOnly[field] = struct{}{}
		}
	}
}

// ClearOnNil checks if a field should be cleared when a nil value is encountered.
// Returns true if the field is configured for nil clearing, false otherwise.
func (o *Options) ClearOnNil(field string) bool {
	if o.clearOnNilFields == nil {
		return false
	}
	_, ok := o.clearOnNilFields[field]
	return ok
}

// IgnoreSetZero checks if zero values should be ignored for a specific field.
// Returns true if the field is configured to ignore zero values, false otherwise.
func (o *Options) IgnoreSetZero(field string) bool {
	if o.ignoreSetZeroFields == nil {
		return false
	}
	_, ok := o.ignoreSetZeroFields[field]
	return ok
}

// CanSetZero checks if zero values are allowed to be set for a specific field.
// This is the inverse of IgnoreSetZero for more intuitive usage.
func (o *Options) CanSetZero(field string) bool {
	return !o.IgnoreSetZero(field)
}

// CanSetField determines if a field should be included in binding operations.
// It respects both the keep-only list and ignore list configurations.
// Returns true if the field should be processed, false if it should be skipped.
func (o *Options) CanSetField(field string) bool {
	if o.keepFieldsOnly != nil {
		_, ok := o.keepFieldsOnly[field]
		return ok
	}
	if o.ignoreFields == nil {
		return true
	}
	_, ok := o.ignoreFields[field]
	return !ok
}
