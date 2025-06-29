package localization

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
)

type Localization struct {
	translations map[string]map[string]string
}

func New() *Localization {
	l := &Localization{
		translations: make(map[string]map[string]string),
	}
	l.loadTranslations()
	return l
}

func (l *Localization) loadTranslations() {
	languages := []string{"ru", "en"}

	for _, lang := range languages {
		filePath := filepath.Join("localization", "locales", fmt.Sprintf("%s.json", lang))
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatalf("Failed to read translation file %s: %v", filePath, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			log.Fatalf("Failed to parse translation file %s: %v", filePath, err)
		}

		l.translations[lang] = translations
	}
}

func (l *Localization) Get(lang, key string, args ...interface{}) string {
	if l.translations[lang] == nil {
		lang = "ru" // Default fallback
	}

	template, exists := l.translations[lang][key]
	if !exists {
		return fmt.Sprintf("Missing translation: %s.%s", lang, key)
	}

	if len(args) > 0 {
		return fmt.Sprintf(template, args...)
	}

	return template
}

func (l *Localization) GetSupportedLanguages() []string {
	var languages []string
	for lang := range l.translations {
		languages = append(languages, lang)
	}
	return languages
}
