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
/// @return 1 if user clicked OK, 2 if user clicked Copy Path, 0 otherwise
int showConfigValidationErrorAlert(const char *errorMessage, const char *configPath);

/// Show a config onboarding alert for new users
/// @param configPath The default config path that will be created
/// @return 1 if user clicked Create Config, 2 if user clicked Use Defaults, 3 if user clicked Quit
int showConfigOnboardingAlert(const char *configPath);

/// Show a macOS notification with a title and message
/// Uses UNUserNotificationCenter when running as an app bundle, falls back to osascript otherwise
/// @param title The notification title
/// @param message The notification message
void showNotification(const char *title, const char *message);

#endif /* ALERT_H */
