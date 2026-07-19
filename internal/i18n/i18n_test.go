package i18n

import (
	"reflect"
	"sort"
	"testing"
)

func TestLocaleKeysMatch(t *testing.T) {
	english, err := Load(EnglishUS)
	if err != nil {
		t.Fatal(err)
	}
	traditionalChinese, err := Load(TraditionalChinese)
	if err != nil {
		t.Fatal(err)
	}
	englishKeys := english.Keys()
	traditionalChineseKeys := traditionalChinese.Keys()
	sort.Strings(englishKeys)
	sort.Strings(traditionalChineseKeys)
	if !reflect.DeepEqual(englishKeys, traditionalChineseKeys) {
		t.Fatalf("locale keys differ\nen-US: %v\nzh-TW: %v", englishKeys, traditionalChineseKeys)
	}
	for _, key := range englishKeys {
		if english.Message(key) == "" || traditionalChinese.Message(key) == "" {
			t.Fatalf("locale message %q must not be empty", key)
		}
	}
}

func TestLocaleNamesAreExactCase(t *testing.T) {
	for _, locale := range []string{"en-us", "zh-tw", "EN-US", ""} {
		if _, err := Load(locale); err == nil {
			t.Fatalf("Load(%q) unexpectedly succeeded", locale)
		}
	}
}
