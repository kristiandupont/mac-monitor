void initCocoaApp(void);
void setupStatusItem(const char* tooltip);
void runCocoaApp(void);
void quitCocoaApp(void);

// Load the single fan base image (white @2x PNG). Call after setupStatusItem.
void loadBaseImage(const unsigned char* data, int len);
// Set rotation angle (degrees) and sRGB tint color. GPU-composited.
void setIconFrame(float angleDeg, float r, float g, float b);

void addMenuItemCStr(const char* title, int itemID);
void addMenuSeparatorItem(void);
