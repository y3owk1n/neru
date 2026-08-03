#ifndef SYSTRAY_H
#define SYSTRAY_H

#include <stdbool.h>

void NeruRegisterSystray(void);
void NeruNativeLoop(void);
void NeruNativeLoopHeadless(void);
void NeruQuit(void);

void NeruSetIcon(const char *iconBytes, int length, bool isTemplate);
void NeruSetTitle(const char *title);
void NeruSetTooltip(const char *tooltip);

void NeruAddMenuItem(int menuId, const char *title, short disabled, short checked);
void NeruAddSubMenuItem(int parentId, int menuId, const char *title, short disabled, short checked);
void NeruAddSeparator(int parentId);
void NeruHideMenuItem(int menuId);
void NeruShowMenuItem(int menuId);
void NeruSetItemChecked(int menuId, short checked);
void NeruSetItemDisabled(int menuId, short disabled);
void NeruSetItemTitle(int menuId, const char *title);

#endif
