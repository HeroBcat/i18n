package main

import (
	"log"

	"golang.org/x/text/language"

	"github.com/HeroBcat/i18n/i18n"
)

func main() {
	i18n.SetLanguage(language.AmericanEnglish)
	i18n.Print.CmdCommonFlagsVUsage()
	log.Println(i18n.CmdCommonFlagsVUsage())
	log.Println("--------------------")

	i18n.SetLanguage(language.SimplifiedChinese)
	i18n.Print.DocsTitleSeeAlso()
	log.Println(i18n.DocsTitleSeeAlso())
	log.Println("--------------------")

	i18n.SetLanguage(language.BritishEnglish)
	i18n.Print.CmdCommonFlagsVUsage()
	log.Println(i18n.CmdCommonFlagsVUsage())
}
