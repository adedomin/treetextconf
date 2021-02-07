# Tree Text Conf

Simple Configuration Format that tries to be easy to use and understand at a glance.
Unlike other formats, TreeTextConf does not have any types other than ascii-like text data.

### Example

```
Key: Value
List of Values:
  1
  2
  3
  4
:
# this is a comment, empty lines are also ignored
# unless they are escaped with a content begin marker (')
'
'# this is not a comment

':
  a list with no name-value
:

':
  nested:
    list:
      type
    :
  :
:

# to escape a list start delimiter (:)
# use a content end marker (')
this is not a list of values:'
```

### Usage

```
// Where file contains:
// something: something else
myConfigFile, err := os.Open("file")
parser, err := NewParser(myConfigFile)
if err != nil {
	panic(err)
}

// Returns a root config element with a name of __root__
// your configuration values are contained in config.value[]
config, err := parser.ParseConfig()
if err != nil {
	panic(err)
}

fmt.Println(config.value[0].name)
// output: something\n
fmt.Println(config.value[0].value[0].name)
// output: something else\n

// Type of treetextconf.Config
type Config struct {
	name string
	value []*Config
}
```


### Todo

  1. More comprehensive tests.
  2. Better documentation
  3. Serialization functions

### Inspired by

[Deco - Delimiter Collision Free Format](https://github.com/Enhex/Deco)
