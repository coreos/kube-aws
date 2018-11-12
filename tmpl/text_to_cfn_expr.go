package tmpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func TextToCfnExprTokens(src string) []json.RawMessage {
	tokens := []json.RawMessage{}
	i := 0
	strStart := -1

	finishStr := func() {
		if strStart != -1 {
			bt, err := json.Marshal(src[strStart:i])
			if err != nil {
				panic(err)
			}
			tokens = append(tokens, json.RawMessage(bt))
			strStart = -1
		}
	}

	readExpr := func() {
		reader := strings.NewReader(src[i:])
		remainings := int64(len(src[i:]))
		dec := json.NewDecoder(reader)
		expr := ""
		var j int64
		for {
			t, err := dec.Token()
			if err == io.EOF {
				break
			}
			r := dec.Buffered().(*bytes.Reader)
			if err != nil {
				break
			}
			j = remainings - r.Size()
			//fmt.Printf("%T: %v %v %d %v\n", t, t, err, j, dec.More())
			expr = expr + fmt.Sprintf("%v", t)
			if t == "}" && !dec.More() {
				break
			}
		}
		tokens = append(tokens, json.RawMessage(src[i:i+int(j)]))
		i = i + int(j)
	}

	starts := []string{`{"Ref":`, `{"Fn::`}

Loop:
	for i < len(src) {
		if src[i] == '{' {
			for _, start := range starts {
				peek := src[i : i+len(start)]
				//fmt.Println(peek)
				if len(src) >= len(start) && peek == start {
					finishStr()

					readExpr()
					continue Loop
				}
			}
		}
		if strStart == -1 {
			strStart = i
		}
		i++
	}
	finishStr()
	return tokens
}

//func TextToCfnExpr(src string) string {
//	tokens := TextToCfnExprTokens(src)
//	return fmt.Sprintf(`{"Fn::Join": ["", [%s]]}`, strings.Join(tokens, ", "))
//}
