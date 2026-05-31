void initCocoaApp(void);
void setupStatusItem(const char* tooltip);
void runCocoaApp(void);
void quitCocoaApp(void);

// Load the single fan base image (white @2x PNG). Call after setupStatusItem.
void loadBaseImage(const unsigned char* data, int len);
// Set rotation angle (degrees) and sRGB tint color. GPU-composited.
void setIconFrame(float angleDeg, float r, float g, float b);

void addMenuItemCStr(const char* title, int itemID);
void addCheckboxMenuItemCStr(const char* title, int itemID, int checked);
void setMenuItemChecked(int itemID, int checked);
void addMenuSeparatorItem(void);

// NSUserDefaults persistence helpers.
void saveUserDefaultBool(const char* key, int value);
int  loadUserDefaultBool(const char* key, int defaultValue);

// Launch-at-login via SMAppService (requires running inside an .app bundle).
void setLaunchAtLogin(int enable);
int  getLaunchAtLogin(void);
