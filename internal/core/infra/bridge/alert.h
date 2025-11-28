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

#endif /* ALERT_H */
