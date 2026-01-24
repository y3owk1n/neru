## The progress

- Started with neovim (I don't know when), and found that it's amazingly nice
- Touched into the browser land of keyboard navigation (vimium, surfingkeys, tridactl, ...)
- Wonder why not these can be also apply for the macOS?
- Discovers vimac (eventually homerow), but with upgrade to paid banners every now and then
- Discovers hammerspoon, and its cool that i can do most of these in hammerspoon (but it feels slow)
- Discovers mouseless, but subscription shooo me away...
- Learning about go and thought, why not try to build this thing with it?
- And here we are.

### Story

The first version was almost identical to vimium or homerow, where accessibility drives almost everything. But along the way, i figured that the accessibility doesn't cover all cases, especially for electron apps.
There's a need for some workaround to make it work for Firefox based, Chromium based, and Electron based apps, that is also why the reason of `X app does not work` issue in vimac or homerow. What's worst is that
even macOS native app itself sometimes lack of accessibility access (face palm). I put a lot of effort and go through homerow's issues to make most of it work here. I try not to coded apps support in the program,
but provide good enough defaults and users are free to customise whatever that they want to support. But still it's troublesome.

Then i started to wonder if there's a better way that we can navigate with keyboard but without the need of complex accessibility edge cases? The answer is grid (just like warpd or mouseless), that provides a static
grid to you (customisable), wherever you selected, it will click there. Now we dont have to care about accessibility access or even targeted delay to react against browser contents (where a click will causes the whole
content to change). And I am now fully in the grid mode even when I still keep support for accessibility.

### Quick demo that how i use it

https://github.com/user-attachments/assets/4504b17d-529a-4485-99b5-aaa04873a29e
