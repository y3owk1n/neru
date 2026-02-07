//
//  alert.h
//  Neru
//
//  Copyright © 2025 Neru. All rights reserved.
//

#ifndef ALERT_H
#define ALERT_H

#import <Foundation/Foundation.h>

#pragma mark - Alert Functions

/// Show a config validation error alert with error details and config path
/// @param errorMessage The error message to display
/// @param configPath The path to the config file
/// @return 1 if user clicked OK, 2 if user clicked Copy, 0 otherwise
int showConfigValidationErrorAlert(const char *errorMessage, const char *configPath);

/// Show a macOS notification with a title and message
/// Uses osascript to display a native macOS notification (works for CLI tools)
/// @param title The notification title
/// @param message The notification message
void showNotification(const char *title, const char *message);

/// Show a success alert popup with a title and message
/// Uses NSAlert to display a native modal dialog
/// @param title The alert title
/// @param message The alert message
void showSuccessAlert(const char *title, const char *message);

#endif /* ALERT_H */
