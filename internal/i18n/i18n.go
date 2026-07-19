package i18n

import (
	"encoding/json"
	"fmt"

	localefiles "github.com/PastureStack/storage-plugins/locales"
)

const (
	EnglishUS          = "en-US"
	TraditionalChinese = "zh-TW"
)

type Catalog struct {
	locale   string
	messages map[string]string
}

func Load(locale string) (Catalog, error) {
	if locale != EnglishUS && locale != TraditionalChinese {
		return Catalog{}, fmt.Errorf("unsupported locale")
	}
	data, err := localefiles.FS.ReadFile(locale + ".json")
	if err != nil {
		return Catalog{}, fmt.Errorf("load locale")
	}
	messages := map[string]string{}
	if err := json.Unmarshal(data, &messages); err != nil {
		return Catalog{}, fmt.Errorf("decode locale")
	}
	return Catalog{locale: locale, messages: messages}, nil
}

func (c Catalog) Locale() string { return c.locale }

func (c Catalog) Message(code string) string {
	if message, ok := c.messages[code]; ok {
		return message
	}
	if message, ok := c.messages["internal-error"]; ok {
		return message
	}
	return "The request could not be completed."
}

func (c Catalog) Keys() []string {
	keys := make([]string, 0, len(c.messages))
	for key := range c.messages {
		keys = append(keys, key)
	}
	return keys
}
