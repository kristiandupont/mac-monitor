void initCocoaApp(void);
void setupStatusItem(const char* tooltip);
void runCocoaApp(void);
void quitCocoaApp(void);

// Called once before runCocoaApp; count = 2*4 = 8 (theme × color-step).
void preloadColorImagesInit(int count);
// Load one base image by color index. PNG data is @2x (44px, declared as 22pt).
void loadColorImage(int idx, const unsigned char* data, int len);
// Set icon color variant and rotation angle (degrees). Rotation is GPU-composited —
// does not repaint the button's backing layer.
void setIconFrame(int colorIdx, float angleDeg);

void addMenuItemCStr(const char* title, int itemID);
void addMenuSeparatorItem(void);
