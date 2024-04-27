//go:build ignore

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"tinygo.org/x/bluetooth"
)

type Characteristic struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
	UUID       string `json:"uuid"`
	Source     string `json:"source"`
}

func (c Characteristic) VarName() string {
	str := strings.ReplaceAll(c.Name, "Characteristic", "")

	// Remove non-alphanumeric characters.
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	str = nonAlphanumericRegex.ReplaceAllString(str, "")

	str = cases.Title(language.Und, cases.NoLower).String(str)
	return strings.ReplaceAll(str, " ", "")
}

func (c Characteristic) UUIDFunc() string {
	if len(c.UUID) == 4 {
		return "New16BitUUID(0x" + c.UUID + ")"
	}
	uuid, err := bluetooth.ParseUUID(strings.ToLower(c.UUID))
	if err != nil {
		panic(err)
	}
	b := uuid.Bytes()
	bs := hex.EncodeToString(b[:])
	bss := ""
	for i := 0; i < len(bs); i += 2 {
		bss = "0x" + bs[i:i+2] + "," + bss
	}
	return "NewUUID([16]byte{" + bss + "})"
}

func dedupCharacteristics(characteristics []Characteristic) []Characteristic {
	// Group characteristics by name.
	byName := make(map[string][]Characteristic)
	for _, c := range characteristics {
		byName[c.Name] = append(byName[c.Name], c)
	}

	var newCharacteristics []Characteristic

	// Find duplicate characteristics and rename them.
	for name, cs := range byName {
		for i, c := range cs {
			if len(cs) > 1 {
				c.Name = fmt.Sprintf("%s %d", name, i+1)
			}
			newCharacteristics = append(newCharacteristics, c)
		}
	}

	return newCharacteristics
}

func main() {
	jsonFile, err := os.Open("bluetooth-numbers-database/v1/characteristic_uuids.json")
	if err != nil {
		fmt.Println(err)
	}

	defer jsonFile.Close()

	data, _ := ioutil.ReadAll(jsonFile)

	var characteristics []Characteristic
	json.Unmarshal(data, &characteristics)

	characteristics = dedupCharacteristics(characteristics)

	f, err := os.Create("characteristic_uuids.go")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	packageTemplate := template.Must(template.New("").Parse(tmpl))

	packageTemplate.Execute(f, struct {
		Timestamp       time.Time
		Characteristics []Characteristic
	}{
		Timestamp:       time.Now(),
		Characteristics: characteristics,
	})
}

var tmpl = `// Code generated by bin/gen-characteristic-uuids; DO NOT EDIT.
// This file was generated on {{.Timestamp}} using the list of standard characteristics UUIDs from
// https://github.com/NordicSemiconductor/bluetooth-numbers-database/blob/master/v1/characteristics_uuids.json
//
package bluetooth

var (
{{ range .Characteristics }}
	// CharacteristicUUID{{.VarName}} - {{.Name}}
	CharacteristicUUID{{.VarName}} = {{.UUIDFunc}}
{{ end }}
)
`