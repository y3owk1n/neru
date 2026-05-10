#ifndef TEXTINPUT_H
#define TEXTINPUT_H

#import <Foundation/Foundation.h>

typedef void (*TextInputQueryCallback)(const char *query, void *userData);
typedef void (*TextInputControlCallback)(void *userData);

int NeruStartHintSearchTextInput(
    TextInputQueryCallback queryCallback,
    TextInputControlCallback confirmCallback,
    TextInputControlCallback cancelCallback,
    int x,
    int y,
    int width,
    int height,
    void *userData);

void NeruStopHintSearchTextInput(void);

#endif
