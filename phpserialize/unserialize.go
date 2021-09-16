package phpserialize

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/eligundry/phpsessgo/phptype"
)

const UNSERIALIZABLE_OBJECT_MAX_LEN = 10 * 1024 * 1024 * 1024

func UnSerialize(s string) (phptype.Value, error) {
	decoder := NewUnserializer(s)
	decoder.SetDecodeFunc(DecodeFunc(UnSerialize))
	return decoder.Decode()
}

type Unserializer struct {
	source     string
	r          *strings.Reader
	lastErr    error
	DecodeFunc DecodeFunc
}

func NewUnserializer(data string) *Unserializer {
	return &Unserializer{
		source: data,
	}
}

func (self *Unserializer) SetReader(r *strings.Reader) {
	self.r = r
}

func (self *Unserializer) SetDecodeFunc(f DecodeFunc) {
	self.DecodeFunc = f
}

func (self *Unserializer) Decode() (phptype.Value, error) {
	if self.r == nil {
		self.r = strings.NewReader(self.source)
	}

	var value phptype.Value

	if token, _, err := self.r.ReadRune(); err == nil {
		switch token {
		default:
			self.saveError(fmt.Errorf("phpserialize: Unknown token %#U", token))
		case TOKEN_NULL:
			value = self.decodeNull()
		case TOKEN_BOOL:
			value = self.decodeBool()
		case TOKEN_INT:
			value = self.decodeNumber(false)
		case TOKEN_FLOAT:
			value = self.decodeNumber(true)
		case TOKEN_STRING:
			value = self.decodeString(DELIMITER_STRING_LEFT, DELIMITER_STRING_RIGHT, true)
		case TOKEN_ARRAY:
			value = self.decodeArray()
		case TOKEN_OBJECT:
			value = self.decodeObject()
		case TOKEN_OBJECT_SERIALIZED:
			value = self.decodeSerialized()
		case TOKEN_REFERENCE, TOKEN_REFERENCE_OBJECT:
			value = self.decodeReference()
		case TOKEN_SPL_ARRAY:
			value = self.decodeSplArray()

		}
	}

	return value, self.lastErr
}

func (self *Unserializer) decodeNull() phptype.Value {
	self.expect(SEPARATOR_VALUES)
	return nil
}

func (self *Unserializer) decodeBool() phptype.Value {
	var (
		raw rune
		err error
	)
	self.expect(SEPARATOR_VALUE_TYPE)

	if raw, _, err = self.r.ReadRune(); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Error while reading bool value: %v", err))
	}

	self.expect(SEPARATOR_VALUES)
	return raw == '1'
}

func (self *Unserializer) decodeNumber(isFloat bool) phptype.Value {
	var (
		raw string
		err error
		val phptype.Value
	)
	self.expect(SEPARATOR_VALUE_TYPE)

	if raw, err = self.readUntil(SEPARATOR_VALUES); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Error while reading number value: %v", err))
	} else {
		if isFloat {
			if val, err = strconv.ParseFloat(raw, 64); err != nil {
				self.saveError(fmt.Errorf("phpserialize: Unable to convert %s to float: %v", raw, err))
			}
		} else {
			if val, err = strconv.Atoi(raw); err != nil {
				self.saveError(fmt.Errorf("phpserialize: Unable to convert %s to int: %v", raw, err))
			}
		}
	}

	return val
}

func (self *Unserializer) decodeString(left, right rune, isFinal bool) phptype.Value {
	var (
		err     error
		val     phptype.Value
		strLen  int
		readLen int
	)

	strLen = self.readLen()
	self.expect(left)

	if strLen > 0 {
		buf := make([]byte, strLen, strLen)
		if readLen, err = self.r.Read(buf); err != nil {
			self.saveError(fmt.Errorf("phpserialize: Error while reading string value: %v", err))
		} else {
			if readLen != strLen {
				self.saveError(fmt.Errorf("phpserialize: Unable to read string. Expected %d but have got %d bytes", strLen, readLen))
			} else {
				val = string(buf)
			}
		}
	}

	self.expect(right)
	if isFinal {
		self.expect(SEPARATOR_VALUES)
	}
	return val
}

func (self *Unserializer) decodeArray() phptype.Value {
	var arrLen int
	val := make(phptype.Array)

	arrLen = self.readLen()
	self.expect(DELIMITER_OBJECT_LEFT)

	for i := 0; i < arrLen; i++ {
		k, errKey := self.Decode()
		v, errVal := self.Decode()

		if errKey == nil && errVal == nil {
			val[k] = v
			/*switch t := k.(type) {
			default:
				self.saveError(fmt.Errorf("phpserialize: Unexpected key type %T", t))
			case string:
				stringKey, _ := k.(string)
				val[stringKey] = v
			case int:
				intKey, _ := k.(int)
				val[strconv.Itoa(intKey)] = v
			}*/
		} else {
			self.saveError(fmt.Errorf("phpserialize: Error while reading key or(and) value of array"))
		}
	}

	self.expect(DELIMITER_OBJECT_RIGHT)
	return val
}

func (self *Unserializer) decodeObject() phptype.Value {
	val := &phptype.Object{
		ClassName: self.readClassName(),
	}

	rawMembers := self.decodeArray()
	val.Members, _ = rawMembers.(phptype.Array)

	return val
}

func (self *Unserializer) decodeSerialized() phptype.Value {
	val := &phptype.ObjectSerialized{
		ClassName: self.readClassName(),
	}

	rawData := self.decodeString(DELIMITER_OBJECT_LEFT, DELIMITER_OBJECT_RIGHT, false)
	val.Data, _ = rawData.(string)

	if self.DecodeFunc != nil && val.Data != "" {
		var err error
		if val.Value, err = self.DecodeFunc(val.Data); err != nil {
			self.saveError(err)
		}
	}

	return val
}

func (self *Unserializer) decodeReference() phptype.Value {
	self.expect(SEPARATOR_VALUE_TYPE)
	if _, err := self.readUntil(SEPARATOR_VALUES); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Error while reading reference value: %v", err))
	}
	return nil
}

func (self *Unserializer) expect(expected rune) {
	if token, _, err := self.r.ReadRune(); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Error while reading expected rune %#U: %v", expected, err))
	} else if token != expected {
		self.saveError(fmt.Errorf("phpserialize: Expected %#U but have got %#U", expected, token))
	}
}

func (self *Unserializer) readUntil(stop rune) (string, error) {
	var (
		token rune
		err   error
	)
	buf := bytes.NewBuffer([]byte{})

	for {
		if token, _, err = self.r.ReadRune(); err != nil || token == stop {
			break
		} else {
			buf.WriteRune(token)
		}
	}

	return buf.String(), err
}

func (self *Unserializer) readLen() int {
	var (
		raw string
		err error
		val int
	)
	self.expect(SEPARATOR_VALUE_TYPE)

	if raw, err = self.readUntil(SEPARATOR_VALUE_TYPE); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Error while reading lenght of value: %v", err))
	} else {
		if val, err = strconv.Atoi(raw); err != nil {
			self.saveError(fmt.Errorf("phpserialize: Unable to convert %s to int: %v", raw, err))
		} else if val > UNSERIALIZABLE_OBJECT_MAX_LEN {
			self.saveError(fmt.Errorf("phpserialize: Unserializable object length looks too big(%d). If you are sure you wanna unserialise it, please increase UNSERIALIZABLE_OBJECT_MAX_LEN const", val))
			val = 0
		}
	}
	return val
}

func (self *Unserializer) readClassName() (res string) {
	rawClass := self.decodeString(DELIMITER_STRING_LEFT, DELIMITER_STRING_RIGHT, false)
	res, _ = rawClass.(string)
	return
}

func (self *Unserializer) saveError(err error) {
	if self.lastErr == nil {
		self.lastErr = err
	}
}

func (self *Unserializer) decodeSplArray() phptype.Value {
	var err error
	val := &phptype.PhpSplArray{}

	self.expect(SEPARATOR_VALUE_TYPE)
	self.expect(TOKEN_INT)

	flags := self.decodeNumber(false)
	if flags == nil {
		self.saveError(fmt.Errorf("phpserialize: Unable to read flags of SplArray"))
		return nil
	}
	val.Flags = flags.(int)

	if val.Array, err = self.Decode(); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Can't parse SplArray: %v", err))
		return nil
	}

	self.expect(SEPARATOR_VALUES)
	self.expect(TOKEN_SPL_ARRAY_MEMBERS)
	self.expect(SEPARATOR_VALUE_TYPE)

	if val.Properties, err = self.Decode(); err != nil {
		self.saveError(fmt.Errorf("phpserialize: Can't parse properties of SplArray: %v", err))
		return nil
	}

	return val
}
