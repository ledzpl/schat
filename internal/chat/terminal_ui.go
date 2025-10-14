package chat

import "sync"

const (
	seqSaveCursor    = "\0337\033[s"
	seqRestoreCursor = "\033[u\0338"
	seqCursorHome    = "\033[H"
	seqClearLine     = "\033[2K"
	seqInsertLine    = "\033[1L"
	seqClearScreen   = "\033[2J"
)

type terminalUI struct {
	writer *sessionWriter

	statusOnce sync.Once
	statusErr  error
}

func newTerminalUI(writer *sessionWriter) *terminalUI {
	return &terminalUI{writer: writer}
}

func (ui *terminalUI) ClearScreen() error {
	return ui.writer.writeString(seqClearScreen + seqCursorHome)
}

func (ui *terminalUI) DisplayControlAck(label string) error {
	return ui.writer.writeString("\r\033[K" + label + "\r\n")
}

func (ui *terminalUI) DisplayMessage(msg string) error {
	return ui.writer.writeString("\r\033[K" + msg + "\r\n")
}

func (ui *terminalUI) UpdatePrompt(header, line string) error {
	if err := ui.ensureStatusLine(); err != nil {
		return err
	}
	if err := ui.renderStatus(header); err != nil {
		return err
	}
	return ui.writer.writeString("\r> " + line + "\033[K")
}

func (ui *terminalUI) ensureStatusLine() error {
	ui.statusOnce.Do(func() {
		ui.statusErr = ui.writer.writeString(seqSaveCursor + seqCursorHome + seqInsertLine + seqRestoreCursor)
	})
	return ui.statusErr
}

func (ui *terminalUI) renderStatus(text string) error {
	return ui.writer.writeString(seqSaveCursor + seqCursorHome + seqClearLine + text + seqRestoreCursor)
}
