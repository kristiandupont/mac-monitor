void initCocoaApp(void);
void setupStatusItem(const char* tooltip);
void runCocoaApp(void);
void quitCocoaApp(void);

// Called once before runCocoaApp; allocates the image table.
void preloadImagesInit(int count);
// Load one frame by index from a PNG byte buffer.
void loadImageAtIndex(int idx, const unsigned char* data, int len);
// Swap the visible icon to a pre-loaded image — no PNG decode.
void setIconIndex(int idx);

void addMenuItemCStr(const char* title, int itemID);
void addMenuSeparatorItem(void);
