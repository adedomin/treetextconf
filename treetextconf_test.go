/*
 * Copyright (c) 2021 Anthony DeDominic <adedomin@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package treetextconf

import (
	"reflect"
	"strings"
	"testing"
)

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a == b {
		return
	}
	t.Errorf("Received %v (type %v), Expected %v (type %v)", a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

func TestNewParser(t *testing.T) {
	hlimit := HeightLimit(-1)
	_, err := NewParser(strings.NewReader("test: 123"), hlimit)

	if err == nil {
		t.Error("setting a negative height limit lest than 0 should have errored.")
	} else if err.Error() != "Height limit must be greater than 0." {
		t.Errorf("Unexpected error: %s", err)
	}

	slimit := SizeLimit(-1)
	_, err = NewParser(strings.NewReader("test: 123"), slimit)

	if err == nil {
		t.Error("setting a size limit less than 0 should have errored.")
	} else if err.Error() != "Size limit must be greater than 0." {
		t.Errorf("Unexpected error: %s", err)
	}

	hlimit = HeightLimit(10)
	slimit = SizeLimit(256)
	var parser *Parser
	parser, err = NewParser(strings.NewReader("test: 123"), hlimit, slimit)

	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	assertEqual(t, parser.heightLimit, 10)
	assertEqual(t, parser.sizeLimit, 256)
}

func setupAndRunParser(teststr string) ([]*Config, error) {
	parser, err := NewParser(strings.NewReader(teststr))
	if err != nil {
		return nil, err
	}
	config, err := parser.ParseConfig()
	if err != nil {
		return nil, err
	}

	return config.value, nil
}

func TestParseConfig(t *testing.T) {
	// Basic Name: Value pair
	conf, err := setupAndRunParser("test: 123")
	if err != nil {
		t.Error(err)
	} else {
		assertEqual(t, conf[0].name, "test")
		assertEqual(t, conf[0].value[0].name, "123")
	}

	// Escaped Name: Value Pair
	conf, err = setupAndRunParser("test: 123:\n:\n")
	if err != nil {
		t.Error(err)
	} else {
		assertEqual(t, conf[0].name, "test: 123")
		assertEqual(t, len(conf[0].value), 0)
	}

	// Single Value
	conf, err = setupAndRunParser("test 123")
	if err != nil {
		t.Error(err)
	} else {
		assertEqual(t, conf[0].name, "test 123")
		assertEqual(t, len(conf[0].value), 0)
	}

	// Multiple Value
	conf, err = setupAndRunParser("test 123:\n  xyz\n  abc\n:")
	if err != nil {
		t.Error(err)
	} else {
		assertEqual(t, conf[0].name, "test 123")
		assertEqual(t, len(conf[0].value), 2)
		assertEqual(t, conf[0].value[0].name, "xyz")
		assertEqual(t, conf[0].value[1].name, "abc")
	}

	// leading whitespace, escaping compound type open, content start and end delimiter
	conf, err = setupAndRunParser("'    four spaces:'\nanother element:'\n'# not a comment\n''\nx:'")
	if err != nil {
		t.Error(err)
	} else {
		assertEqual(t, conf[0].name, "    four spaces:")
		assertEqual(t, conf[1].name, "another element:")
		assertEqual(t, conf[2].name, "# not a comment")
		assertEqual(t, conf[3].name, "")
		assertEqual(t, conf[4].name, "x:")
	}

	// Content start delimiter only
	conf, err = setupAndRunParser("'\n     '")
	if err != nil {
		t.Error(err)
	} else {
		assertEqual(t, conf[0].name, "")
		assertEqual(t, conf[1].name, "")
	}

	// Missing compound close
	conf, err = setupAndRunParser("test 123:\n  xyz\n  abc\n")
	if err == nil { t.Errorf("Expected an error for missing (:) for closing a compound type.") }

	// Too many closes
	conf, err = setupAndRunParser("test 123:\n  xyz\n:\n  abc\n:")
	if err == nil { t.Errorf("Expected an error for too many closing (:).") }

	// Test Height Limit
	hlimit := HeightLimit(3)
	parser, err := NewParser(strings.NewReader("a:\nb:\nc:\nd:\n:\n:\n:\n:"), hlimit)
	if err != nil {
		t.Error(err)
	}
	_, err = parser.ParseConfig()
	if err == nil {
		t.Errorf("Expected Error due to limit on the height of the tree being exceeded.")
	}

	// Test Height Limit
	hlimit = HeightLimit(4)
	parser, err = NewParser(strings.NewReader("a:\nb:\nc:\nd:\n:\n:\n:\n:"), hlimit)
	if err != nil {
		t.Error(err)
	}
	_, err = parser.ParseConfig()
	if err != nil {
		t.Error(err)
	}

	// Test Size (in bytes) limit
	slimit := SizeLimit(25)
	parser, err = NewParser(strings.NewReader("a bunch of words\nmore words\n more words"), slimit)
	if err != nil {
		t.Error(err)
	}
	_, err = parser.ParseConfig()
	if err == nil {
		t.Errorf(
			"Expected Error due to input being too large: expected: %d, actual: %d.",
			25, parser.size,
		)
	}
}
