package cli

import "github.com/y3owk1n/neru/internal/domain"

// VimCmd is the CLI vim command.
var VimCmd = BuildModeCommand(ModeConfig{
	Mode:  domain.ModeVim,
	Short: "Activate vim mode over the screen's text",
	Long: `Vim mode reads the text in the focused window, or the whole screen with
--capture-scope screen, and puts a cursor on it that vim's keys move.

Motions, counts, text objects, search, registers, macros and dot repeat work
as they do in vim. Marks do not, because every scroll and edit reads the
screen again and a mark would point at other text. Vim mode makes each edit
in the app under the cursor, by clicking the caret into place, selecting with
Shift+Right and typing, then reads the screen again:

  i a I A o O         click the caret into place, then type into the app
                      until Escape
  c d x r J ...       select the text, then erase it or type over it
  p P                 click into place and paste
  u Ctrl+R            the app's own undo and redo
  .                   repeat the last change, with what was typed into the app
  y{motion}           copy the text as read, without touching the app
  Return gc           left click the character under the cursor, as many
                      times as the count up to three, so 2gc double clicks.
                      Vim mode stays open and reads the screen again
  gr gm               right or middle click there, the same way
  gs{motion}          drag across the text, so the app selects it, then leave
  s{chars}{label}     jump to a match of any number of characters by its
                      label, or to the nearest with Return
  ]i [i               move to the next or the previous icon
  Ctrl+W h j k l      move to the container left, below, above or right,
                      within the block the cursor is in until it ends
  Ctrl+W w W          move to the next or the previous container
  ic ac               the container's text, as yic copies a card, and with
                      a count the block around it, as y2ic copies the column
  Ctrl+D Ctrl+U       scroll the app half a page
  Ctrl+F Ctrl+B       scroll the app a page
  Ctrl+E Ctrl+Y       scroll the app two lines, as j and k do past the edge
  zh zl zH zL         scroll the app sideways eight characters, or the
                      count, or half the width read
  zz                  scroll the app until the cursor's line is in the middle
  Ctrl+L              read the screen again
  :q                  leave
  :leftclick [n]      left click at the cursor n times, and :rightclick and
                      :middleclick the other buttons
  :leftdown :leftup   press or release at the cursor, so a motion between
                      them drags, and :rightdown :rightup :middledown
                      :middleup the other buttons
  Escape              cancel what is pending, then leave

Vim mode groups the words of a column, a sidebar or a card into a container,
using the gaps between words and the color behind them. It lays each
container out as its own block, so j, k, w and search stay inside the one you
are reading, and it outlines the one the cursor is in. Containers nest, so a
card sits in a column and the column in the page, and the block around the
cursor's container is outlined fainter.

Vim mode reads the screen twice. A fast read opens the mode at once, and an
accurate read replaces it when it finishes. The status bar says "reading…"
until then. The accurate read also finds icons and pictures with no text,
such as an icon-only button, and puts a □ where each one is, so w, j and s
reach it and Return clicks it. A yank leaves the □ out. The box beside the
cursor shows the word under it as read.

The fast read knows Latin script only. The accurate read finds each line's
script, so it reads Chinese, Japanese, Korean, Thai, Russian and the rest,
and a wide character, as in Chinese, takes two columns.

Examples:
  neru vim                            # Read the focused window
  neru vim --capture-scope screen     # Read the whole screen
  neru vim --cursor-selection-mode hold   # Keep the pointer still until a click`,
})

func init() {
	RootCmd.AddCommand(VimCmd)
}
