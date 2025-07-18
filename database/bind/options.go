package bind

type Options struct {
	ignoreSetZeroFields map[string]struct{}
	clearOnNilFields    map[string]struct{}
	ignoreFields        map[string]struct{}
	keepFieldsOnly      map[string]struct{}
}

func NewBindOptions(options ...Option) *Options {
	defaults := &Options{}
	for _, opt := range options {
		opt(defaults)
	}
	return defaults
}

type Option func(*Options)

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

func (o *Options) ClearOnNil(field string) bool {
	if o.clearOnNilFields == nil {
		return false
	}
	_, ok := o.clearOnNilFields[field]
	return ok
}

func (o *Options) IgnoreSetZero(field string) bool {
	if o.ignoreSetZeroFields == nil {
		return false
	}
	_, ok := o.ignoreSetZeroFields[field]
	return ok
}

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
