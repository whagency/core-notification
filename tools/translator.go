package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/whagency/core-common/logger"
)

type Translator struct {
	defaultLang string
	templates   map[string]map[string]*template.Template
}

func NewTranslator(path string, defaultLang string) (*Translator, error) {
	t := &Translator{
		defaultLang: defaultLang,
		templates:   make(map[string]map[string]*template.Template),
	}

	if err := t.load(path); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Translator) load(path string) error {
	files, errFiles := filepath.Glob(filepath.Join(path, "*.json"))
	if errFiles != nil {
		return errFiles
	}

	for _, file := range files {
		lang := strings.TrimSuffix(filepath.Base(file), ".json")

		data, errRead := os.ReadFile(file)
		if errRead != nil {
			return errRead
		}

		var content map[string]string
		if errUnmarshal := json.Unmarshal(data, &content); errUnmarshal != nil {
			return errUnmarshal
		}

		t.templates[lang] = make(map[string]*template.Template)
		for k, text := range content {
			tmpl, errParse := template.New(k).Parse(text)
			if errParse != nil {
				logger.LogCtx(context.Background()).Warnf("[translator][load][lang: %s][key: %s][err: %s]", lang, k, errParse)
				continue
			}
			t.templates[lang][k] = tmpl
		}
	}

	return nil
}

func (t *Translator) T(lang string, key string, data interface{}) string {
	var tmpl *template.Template

	langMap, ok := t.templates[lang]
	if ok {
		tmpl = langMap[key]
	}

	if tmpl == nil {
		if defaultMap, okDefault := t.templates[t.defaultLang]; okDefault {
			tmpl = defaultMap[key]
		}
	}

	if tmpl == nil {
		return key
	}

	var buf bytes.Buffer
	if errExecute := tmpl.Execute(&buf, data); errExecute != nil {
		return key
	}

	return buf.String()
}
