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

type Config struct {
	name string
	value []*Config
}

func (c *Config) addValue(v *Config) {
	c.value = append(c.value, v)
}

type ConfigError struct {
	context string
	line int
	col int
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf(
		"line:%d col:%d Error: %s\n",
		e.line, e.col, e.context,
	)
}

type ParserOptFunc func(p *Parser) error

type ParserOptFuncError string

func (e ParserOptFuncError) Error() string {
	return string(e)
}

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

type Parser struct {
	content *bufio.Reader
	line []byte
	lineno int
	size int
	heightLimit int
	sizeLimit int
}

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
		if height >= p.heightLimit {
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

func (p *Parser) recursiveParse(c *Config, height int) error {
	var err error
	if err = p.checkHeight(height); err != nil {
		return err
	}
	for err = p.nextLine(); err != io.EOF; err = p.nextLine() {
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
				return nil
			} else {
				c.addValue(newConf)
				newConf.name = string(p.line[start:end])
				if err = p.recursiveParse(newConf, height + 1); err != nil {
					return err
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

	if err == io.EOF && height > 0 {
		return &ConfigError{
			context: "Unterminated compound group, not enough ':'",
			line: p.lineno,
			col: 0,
		}
	} else {
		return err
	}
}

func (p *Parser) ParseConfig() (*Config, error) {
	root := &Config{
		name: "__root__",
	}

	err := p.recursiveParse(root, 0)
	if err == nil {
		return root, &ConfigError{
			context: "Too many compound terminators ':'",
			line: p.lineno,
			col: 0,
		}
	} else if err != nil && err != io.EOF {
		return root, err
	} else {
		return root, nil
	}
}

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
