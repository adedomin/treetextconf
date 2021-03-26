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
	"bufio"
	"fmt"
	"io"
	"strings"
)

// A Configuration tree node
// Contains a name and associated nodes with their own names and values
type Config struct {
	name string
	value []*Config
}

// Add a Config to a value of another
func (c *Config) addValue(v *Config) {
	c.value = append(c.value, v)
}

// Error with a message, line and column context
type ConfigError struct {
	context string
	line int
	col int
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf(
		"line:%d col:%d Error: %s",
		e.line, e.col, e.context,
	)
}

// A function that takes a parser being constructed.
// Returns error for invalid options
type ParserOptFunc func(p *Parser) error

type ParserOptFuncError string

func (e ParserOptFuncError) Error() string {
	return string(e)
}

// Sets a User controlled tree height to a new parser
// example: treetextconf.NewParser(file, HeightLimit(10))
func HeightLimit(limit int) ParserOptFunc {
	return func(p *Parser) error {
		if limit > 0 {
			p.heightLimit = limit
			return nil
		} else {
			return ParserOptFuncError("Height limit must be greater than 0.")
		}
	}
}


// Sets a limit on how large (in bytes) a configuration can be.
// example: treetextconf.NewParser(file, SizeLimit(1024 * 16))
func SizeLimit(limit int) ParserOptFunc {
	return func(p *Parser) error {
		if limit > 0 {
			p.sizeLimit = limit
			return nil
		} else {
			return ParserOptFuncError("Size limit must be greater than 0.")
		}
	}
}

// Parser that stores the context, state and a Reader of the file being parsed
type Parser struct {
	content *bufio.Reader
	line []byte
	lineno int
	size int
	heightLimit int
	sizeLimit int
}

// Constructs a new parser with defaults.
// You can configure heightLimit and sizeLimit by passing the functions returned by:
//
//   - HeightLimit() maximum depth the parser will go.
//   - SizeLimit()   maxium size (in bytes) that the parser will read.
func NewParser(content io.Reader, options ...ParserOptFunc) (*Parser, error) {
	h := &Parser{
		content: bufio.NewReader(content),
		lineno: 0,
		size: 0,
		heightLimit: -1,
		sizeLimit: -1,
	}

	for _, option := range options {
		if err := option(h); err != nil {
			return nil, err
		}
	}

	return h, nil
}

func (p *Parser) checkHeight(height int) error {
	if p.heightLimit != -1 {
		if height > p.heightLimit {
			return &ConfigError{
				context: fmt.Sprintf("tree height exceeds limit: %d", p.heightLimit),
				line: p.lineno,
				col: 0,
			}
		}
	}

	return nil
}

func (p *Parser) checkSize() error {
	if p.sizeLimit != -1 {
		if p.size >= p.sizeLimit {
			return &ConfigError{
				context: fmt.Sprintf("size of config exceeds limit: %d", p.sizeLimit),
				line: p.lineno,
				col: 0,
			}
		}
	}

	return nil
}

func (p *Parser) nextLine() error {
	var prefix bool
	var err error
	p.line, prefix, err = p.content.ReadLine()
	if err != nil {
		return err
	}

	p.size += len(p.line)
	if err = p.checkSize(); err != nil {
		return err
	}
	if prefix {
		for prefix {
			var tline []byte
			tline, prefix, err = p.content.ReadLine()
			if err != nil && err != io.EOF {
				return err
			}
			p.size += len(tline)
			if err = p.checkSize(); err != nil {
				return err
			}
			p.line = append(p.line, tline...)
		}
	}

	p.lineno++

	return nil
}

func (p *Parser) iterParse(root *Config) error {
	var err error
	stack := []*Config{root}
	c := stack[len(stack) - 1]
	
out:
	for err = p.nextLine(); err == nil; err = p.nextLine() {
		i := 0
		// find start of content
		for ; i < len(p.line); i++ {
			if p.line[i] != ' ' && p.line[i] != '\t' {
				break
			}
		}
		// skip empty p.lines
		if i == len(p.line) {
			continue
		}

		start := i
		// start content marker
		// escapes leading whitespace and closing colon, e.g. ':
		foundContentStart := false
		if p.line[i] == '\'' {
			i++ // skip content_marker
			start = i
			foundContentStart = true
		// skip comments
		} else if p.line[i] == '#' {
			continue
		}

		// find end
		end := len(p.line)
		endToken := p.line[end-1]
		switch (endToken) {
		case ':':
			end = end - 1
		case '\'':
			if start != end {
				end = end -1
			}
		}
		
		// find name: value pair start (": ")
		nvPairNameEnd := -1
		foundNVPairMaybe := false
		for ; i < end; i++ {
			if p.line[i] == ':' {
				foundNVPairMaybe = true
			} else if foundNVPairMaybe && p.line[i] == ' ' {
				nvPairNameEnd = i - 1
			} else if foundNVPairMaybe {
				foundNVPairMaybe = false
			}
		}

		newConf := &Config{}
		switch (endToken) {
		case ':':
			if start == end && !foundContentStart {
				stack = stack[:len(stack)-1]
				if (len(stack) == 0) {
					break out // too many '\n:'
				}
				c = stack[len(stack)-1]
			} else {
				c.addValue(newConf)
				newConf.name = string(p.line[start:end])
				stack = append(stack, newConf)
				c = newConf
				err = p.checkHeight(len(stack)-1)
				if err != nil {
					break out // tree too big
				}
			}
		default:
			if nvPairNameEnd != -1 {
				newConf.name = string(p.line[start:nvPairNameEnd])
				newNewConf := &Config{
					name: string(p.line[nvPairNameEnd+2:end]),
				}
				newConf.addValue(newNewConf)
				c.addValue(newConf)
			} else {
				newConf.name = string(p.line[start:end])
				c.addValue(newConf)
			}
		}
	}

	if err == io.EOF && len(stack) > 1 {
		return &ConfigError{
			context: "Unterminated compound group, not enough ':'",
			line: p.lineno,
			col: 0,
		}
	} else if len(stack) == 0 {
		return &ConfigError{
			context: "Too many compound terminators ':'",
			line: p.lineno,
			col: 0,
		}
	} else if err != io.EOF {
		return err
	} else {
		return nil
	}
}

// Executes the constructed parser returning a config.
// config.value will contain your configuration file's parsed contents.
// config.name == "__root__" which is the default root node, even if the file
// is empty.
func (p *Parser) ParseConfig() (*Config, error) {
	root := &Config{
		name: "__root__",
	}

	err := p.iterParse(root)
	return root, err
}

// Tool to print what your configuration contains, in pre-order traversal
func DebugPrintConfig(root *Config, depth int) {
	var padding strings.Builder
	for i := 0; i < depth; i++ {
		padding.WriteRune('-')
	}

	fmt.Printf("%s%s\n", padding.String(), root.name)
	for _, v := range root.value {
		DebugPrintConfig(v, depth+1)
	}
}

// TODO: Add Iterator?
