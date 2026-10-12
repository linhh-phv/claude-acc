package app

import (
	"os"

	"github.com/linhh-phv/claude-acc/internal/fsx"
	"github.com/linhh-phv/claude-acc/internal/i18n"
	"github.com/linhh-phv/claude-acc/internal/maincred"
	"github.com/linhh-phv/claude-acc/internal/ui"
)

// cmdLang shows or sets the language of every message.
func cmdLang(args []string) int {
	want := arg(args, 0)
	if want == "" {
		src := i18n.T("lang_src_auto")
		if fsx.NonEmpty(langFile) {
			src = i18n.T("lang_src_file")
		}
		ui.Info(i18n.Tp("lang_current", i18n.Lang, src))
		ui.Info(ui.Dim + i18n.T("lang_howto") + ui.Reset)
		return 0
	}
	switch want {
	case "vi", "en":
		os.WriteFile(langFile, []byte(want+"\n"), 0o644)
		i18n.Lang = want
		ui.Info(ui.Green + i18n.Tp("lang_set", want) + ui.Reset)
	case "auto":
		os.Remove(langFile)
		i18n.Lang = i18n.Detect(langFile)
		ui.Info(ui.Green + i18n.Tp("lang_set_auto", i18n.Lang) + ui.Reset)
	default:
		ui.Die(i18n.T("lang_bad"))
	}
	return 0
}

// cmdRun runs one `claude` under an account without touching any pin.
func cmdRun(args []string) int {
	name := arg(args, 0)
	if name == "" {
		ui.Die(i18n.T("usage_run"))
	}
	args = args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	// The settings env block overrides the process environment (measured), so
	// once something is pinned `run` cannot change the account — say so
	// instead of silently running the wrong one. A directory pin beats even
	// the global one, so it is checked first.
	cwd := fsx.Cwd()
	hpin := repoPin(cwd)
	if hpin != "" && hpin != name {
		ui.Die(i18n.Tp("run_dirpin", hpin, repoFile(cwd), name))
	}
	if pin := pinnedAcc(); pin != "" && hpin == "" && pin != name {
		fix := i18n.Tp("run_fix_acc", name)
		if name == "main" {
			fix = i18n.T("run_fix_main")
		}
		ui.Die(i18n.Tp("run_glbpin", pin, fix))
	}

	if name == "main" {
		return execClaude(maincred.WithoutToken(os.Environ()), args)
	}
	validName(name)
	tok := tokenOf(name)
	return execClaude(append(os.Environ(), envToken+"="+tok), args)
}

// cmdEnv prints the line that switches the CURRENT shell only.
func cmdEnv(args []string) int {
	name := arg(args, 0)
	if name == "" {
		ui.Die(i18n.T("usage_env"))
	}
	if name == "main" {
		ui.Out(unsetLine())
		return 0
	}
	validName(name)
	ui.Out(exportLine(tokenOf(name)))
	return 0
}
