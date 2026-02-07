#ifndef SYSTRAY_H
#define SYSTRAY_H

#include <stdbool.h>

void registerSystray(void);
void nativeLoop(void);
void quit(void);

void setIcon(const char *iconBytes, int length, bool isTemplate);
void setTitle(const char *title);

void add_menu_item(int menuId, const char *title, short disabled, short checked);
void add_sub_menu_item(int parentId, int menuId, const char *title, short disabled, short checked);
void add_separator(int parentId);
void hide_menu_item(int menuId);
void show_menu_item(int menuId);
void set_item_checked(int menuId, short checked);
void set_item_disabled(int menuId, short disabled);
void set_item_title(int menuId, const char *title);

#endif
