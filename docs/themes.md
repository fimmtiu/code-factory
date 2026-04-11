# Terminal themes

We want to add multiple terminal themes as a feature to code-factory, rather than being restricted to the current hard-coded set of styles.

## Requirements

* It should be settable by a `"terminal_theme"` setting in the repository's `.code-factory/settings.json` file.

* We should have three variants to start with: "dark", "light", and "tan" (the default). "tan" is the same as the current styles. "dark" is something that would be clear and readable against a black background. "light" is something that would be clear and readable against a white background. Choose what you feel are reasonable values for the "dark" and "light" styles, and then we'll tweak them afterwards.

* The code should be neatly organized and isolated, with the rest of the app having a clean interface to the terminal theme code which doesn't require it to know anything about colours or themes. The app's awareness of styles should be purely semantic, rather than having to be aware of things like bold and background vs. foreground colours.

* The main `README.md` should be updated to include this as one of the documented settings in `settings.json`. The `code-factory/README.md` should be updated to include `terminal_theme` in the list of settings.

## Notes

* Themes don't only control colours — they control styles as well. We may want different styles for different themes.

* An invalid theme should cause `code-factory` to fail on startup with a helpful message.

* Instead of having global constants for the themes, let's encapsulate this in a theme struct which the app can call to get the styles (e.g., `theme.PrimaryColor()`).
