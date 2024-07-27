package changelog

import "errors"

var ErrMissingPlaceholder = errors.New("changelog file does not contain <!-- INSERT COMMENT -->")
