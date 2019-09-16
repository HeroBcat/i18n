package i18n

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dave/jennifer/jen"
	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
	"gopkg.in/yaml.v2"
)

// plural.Selectf is that it accepts only %d, %f, %g, %e

var (
	defaultLanguageTag  = language.SimplifiedChinese
	curLanguageTag      = defaultLanguageTag
	supportLanguageTags = []language.Tag{language.SimplifiedChinese, language.AmericanEnglish, language.BritishEnglish}
)

var (
	packName       = "i18n"
	yamlPathPrefix = "i18n/locales/"
	generatedPath  = "i18n/i18n.generated.go"
)

var (
	placeholders = []string{""}
)

type content struct {
	Message string    `yaml:"message"`
	Args    int       `yaml:"args"`
	Selectf []selectf `yaml:"selectf"`
	Varf    []varf    `yaml:"varf"`
}

type selectf struct {
	Arg    int           `yaml:"arg"`
	Format string        `yaml:"format"`
	Cases  []interface{} `yaml:"case"`
}

type varf struct {
	Tag     string    `yaml:"tag"`
	Str     string    `yaml:"str"`
	Selectf []selectf `yaml:"selectf"`
}

type contentMap map[string]content

var (
	yamlContentMap contentMap
	yamlMap        map[string]string
)

func getYamlContent() error {

	tagName := curLanguageTag.String()
	yamlContentMap = contentMap{}

	for key, value := range yamlMap {

		if strings.HasSuffix(key, tagName) {
			prefix := cleanup(key, false) + "_"
			tmp := contentMap{}
			err := yaml.Unmarshal([]byte(value), &tmp)
			if err == nil {
				for tmpKey, tmpValue := range tmp {
					yamlContentMap[prefix+tmpKey] = tmpValue
				}
				continue
			}
		}
	}

	return errors.New("*_" + curLanguageTag.String() + ".yaml is not exist")

}

func SetDefaultLanguage() {
	SetLanguage(curLanguageTag)
}

func SetLanguage(languageTag language.Tag) {

	if len(yamlContentMap) == 0 || curLanguageTag != languageTag {

		curLanguageTag = languageTag

		getYamlContent()

		var getSelectfMessages = func(selectfs []selectf) []catalog.Message {

			messages := make([]catalog.Message, 0)

			for _, sel := range selectfs {
				if sel.Format != "" && len(sel.Cases) > 0 {
					messages = append(messages, plural.Selectf(sel.Arg, sel.Format, sel.Cases...))
				}
			}

			return messages

		}

		for _, cm := range yamlContentMap {

			messages := make([]catalog.Message, 0)

			if msg := getSelectfMessages(cm.Selectf); len(msg) > 0 {
				messages = append(messages, msg...)
			} else if len(cm.Varf) > 0 {

				for _, v := range cm.Varf {
					if msg := getSelectfMessages(v.Selectf); len(msg) > 0 {
						messages = append(messages, catalog.Var(v.Tag, msg...))
						messages = append(messages, catalog.String(v.Str))
					}
				}

			}

			if len(messages) == 0 {
				message.SetString(curLanguageTag, cm.Message, cm.Message)
			} else {
				message.Set(curLanguageTag, cm.Message, messages...)
			}

		}
	}
}

func Printf(key string, args ...interface{}) {

	if len(yamlContentMap) == 0 {
		getYamlContent()
	}

	p := message.NewPrinter(curLanguageTag)

	if k, ok := yamlContentMap[key]; ok {
		p.Printf(k.Message, args...)
		fmt.Println()
		return
	}

	p.Printf(key, args...)
	fmt.Println()
}

func SPrintf(key string, args ...interface{}) string {

	if len(yamlContentMap) == 0 {
		getYamlContent()
	}

	if k, ok := yamlContentMap[key]; ok {
		p := message.NewPrinter(curLanguageTag)
		return p.Sprintf(k.Message, args...)
	}
	return ""
}

func Check() {

	files := getLanguageFiles()
	checkMap := make(map[string]map[string]map[string]string, 0) // language - filename - key - message
	for _, tag := range supportLanguageTags {
		checkMap[tag.String()] = make(map[string]map[string]string, 0)
	}

	fileMap := make(map[string]map[string]string, 0)
	for _, file := range files {
		list := strings.Split(cleanup(file, true), "_")
		if len := len(list); len > 1 {
			key := list[len-1]
			filename := cleanup(file, false)
			values := make(contentMap, 0)
			byte, _ := ioutil.ReadFile(file)
			yaml.Unmarshal(byte, &values)

			valueMap := make(map[string]string, 0)
			for k, v := range values {
				valueMap[k] = v.Message
			}
			if value, ok := checkMap[key]; ok {
				value[filename] = valueMap
				fileMap = value
			} else {
				fileMap[filename] = valueMap
			}

			checkMap[key] = fileMap
		}
	}

	var compare = func(tag language.Tag, refTag language.Tag) []string {

		missingKeys := make([]string, 0)

		original := checkMap[tag.String()]
		ref := checkMap[refTag.String()]

		for fileKey := range ref {
			if originalFileKey, ok := original[fileKey]; ok {
				for valueKey := range ref[fileKey] {
					if _, ok := originalFileKey[valueKey]; !ok {
						missingKeys = append(missingKeys, fileKey+"  "+valueKey)
					}
				}
			} else {
				for k := range ref[fileKey] {
					missingKeys = append(missingKeys, fileKey+"  "+k)
				}
			}
		}
		sort.Strings(missingKeys)
		return missingKeys
	}

	tagCount := len(supportLanguageTags)
	if tagCount > 1 {
		for _, tag := range supportLanguageTags {
			for idx := 0; idx < tagCount; idx++ {
				refTag := supportLanguageTags[idx]
				if tag != refTag {
					missingKeys := compare(tag, refTag)
					for _, key := range missingKeys {
						fmt.Printf("[%s -> %s] %s", refTag.String(), tag.String(), key)
						fmt.Println()
					}
					fmt.Println()
				}
			}
		}
	}
}

func Generate() {

	file := jen.NewFile(packName)
	file.Comment("Code generated by " + packName + "/pkg/main.go; DO NOT EDIT.")
	file.Add(jen.Empty())

	dict := make(jen.Dict, 0)

	files := getLanguageFiles()
	for _, filename := range files {
		byte, _ := ioutil.ReadFile(filename)
		filename = cleanup(filename, true)
		dict[jen.Lit(filename)] = jen.Lit(string(byte))
	}

	file.Func().Id("init").Params().Block(
		jen.Id("yamlMap").Op("=").Map(jen.String()).String().Values(dict),
	)
	file.Add(jen.Empty())

	printContext := "printContext"
	file.Var().Id("Print").Op("=").Qual("", "NewPrintContext").Call()
	file.Type().Id(printContext).Struct()
	file.Func().Id("New" + camelString(printContext, false)).Params().Id(printContext).Block(
		jen.Return().Id(printContext).Block(),
	)

	file.Add(jen.Empty())

	vars := make([]string, 0)
	methods := make([]string, 0)

	for _, filename := range files {

		ref := make(contentMap, 0)
		byte, _ := ioutil.ReadFile(filename)
		yaml.Unmarshal(byte, &ref)

		if len(ref) == 0 {
			continue
		}

		prefix := cleanup(filename, false) + "_"

		keys := make([]string, 0)
		for key := range ref {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		codes := make([]jen.Code, 0)
		for _, key := range keys {
			name := camelString(strings.ToLower(prefix+key), true)
			if !containsString(vars, name) {
				vars = append(vars, name)
				codes = append(codes, jen.Id(name).Op("=").Lit(prefix+key))
			}

		}

		file.Var().Defs(codes...)

		for _, key := range keys {
			keyName := camelString(strings.ToLower(prefix+key), true)
			name := camelString(strings.ToLower(prefix+key), false)

			method := name
			if !containsString(methods, method) {
				methods = append(methods, method)
				params := make([]jen.Code, 0)
				args := []jen.Code{jen.Id(keyName)}
				value := ref[key]
				for i := 0; i < value.Args; i++ {
					args = append(args, jen.Id("arg"+strconv.Itoa(i)))
					params = append(params, jen.Id("arg"+strconv.Itoa(i)).Id("interface{}"))
				}

				file.Func().Id(method).Params(params...).String().Block(
					jen.Return(jen.Qual("", "SPrintf").Call(args...)),
				)
				file.Add(jen.Empty())
			}

		}

		for _, key := range keys {
			keyName := camelString(strings.ToLower(prefix+key), true)
			name := camelString(strings.ToLower(prefix+key), false)

			method := "Print" + name
			if !containsString(methods, method) {
				methods = append(methods, method)
				params := make([]jen.Code, 0)
				args := []jen.Code{jen.Id(keyName)}
				value := ref[key]
				for i := 0; i < value.Args; i++ {
					args = append(args, jen.Id("arg"+strconv.Itoa(i)))
					params = append(params, jen.Id("arg"+strconv.Itoa(i)).Id("interface{}"))
				}

				file.Func().Params(jen.Id("p").Id(printContext)).Id(name).Params(params...).Block(
					jen.Qual("", "Printf").Call(args...),
				)
				file.Add(jen.Empty())
			}

		}
	}

	file.Save(generatedPath)
}

func camelString(s string, ignoreFirst bool) string {
	data := make([]byte, 0, len(s))
	j := false
	k := false
	num := len(s) - 1
	for i := 0; i <= num; i++ {
		d := s[i]
		if ignoreFirst && i == 0 {
			data = append(data, d)
			k = true
			continue
		}
		if k == false && d >= 'A' && d <= 'Z' {
			k = true
		}
		if d >= 'a' && d <= 'z' && (j || k == false) {
			d = d - 32
			j = false
			k = true
		}
		if k && d == '_' && num > i && s[i+1] >= 'a' && s[i+1] <= 'z' {
			j = true
			continue
		}
		data = append(data, d)
	}

	result := string(data[:])
	result = strings.Replace(result, "_", "", -1)
	result = strings.Replace(result, "-", "", -1)
	return result
}

func containsString(list []string, str string) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}

	return false
}

func isYamlExt(filename string) bool {
	if ext := filepath.Ext(filename); ext == ".yaml" || ext == ".yml" {
		return true
	}
	return false
}

func cleanup(filename string, keepLanguageTag bool) string {

	filename = strings.Replace(filename, filepath.Dir(filename), "", 1)
	filename = strings.Replace(filename, "/", "", 1)

	return cleanupExt(filename, keepLanguageTag)
}

func cleanupExt(filename string, keepLanguageTag bool) string {

	if !keepLanguageTag {
		names := strings.Split(filename, "_")
		if len := len(names); len > 1 {
			names = names[0 : len-1]
			return strings.Join(names, "_")
		}
	}

	filename = strings.Replace(filename, filepath.Ext(filename), "", 1)
	return filename
}

func getLanguageFiles() []string {

	result := make([]string, 0)

	for _, tag := range supportLanguageTags {

		filepath.Walk(yamlPathPrefix, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			filename := info.Name()
			if isYamlExt(filename) && strings.Contains(filename, tag.String()) {
				result = append(result, path)
			}
			return nil
		})

	}

	return result
}
