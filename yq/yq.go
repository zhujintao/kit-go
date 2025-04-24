package yq

import (
	"bytes"
	"io"
	"strings"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"gopkg.in/op/go-logging.v1"
)

var (
	yamlout, _ = yqlib.FormatFromString("yaml")
	encoder    = yamlout.EncoderFactory()
	yq         = yqlib.NewXMLDecoder(yqlib.XmlPreferences{})
)

func init() {

	yqlib.GetLogger().SetBackend(logging.AddModuleLevel(logging.NewLogBackend(io.Discard, "", 0)))

	yqlib.InitExpressionParser()

}

// pick XML output yaml fromat, xmlPaht get c path .a.b.c
func PickXml(text string, xmlPath string) string {

	reader := bytes.NewReader([]byte(text))
	se := yqlib.NewStreamEvaluator()

	toyaml := bytes.NewBuffer(nil)
	print := yqlib.NewPrinter(encoder, yqlib.NewSinglePrinterWriter(toyaml))
	node, err := yqlib.ExpressionParser.ParseExpression(xmlPath)

	if err != nil {

		return ""
	}

	_, err = se.Evaluate("", reader, node, print, yq)
	if err != nil {

		return ""
	}

	if toyaml.String() == "null\n" {
		return ""
	}

	return strings.TrimSuffix(toyaml.String(), "\n")

}
