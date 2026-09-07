import Gio from 'gi://Gio';
import GLib from 'gi://GLib';
import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';

const BUS_NAME = 'org.neru.Shell';
const OBJECT_PATH = '/org/neru/Shell';
const STATE_SIGNATURE = '(bssiiii)';

const INTERFACE = `
<node>
  <interface name="org.neru.Shell">
    <method name="FocusedWindow">
      <arg type="b" direction="out" name="found"/>
      <arg type="s" direction="out" name="app_id"/>
      <arg type="s" direction="out" name="title"/>
      <arg type="i" direction="out" name="x"/>
      <arg type="i" direction="out" name="y"/>
      <arg type="i" direction="out" name="width"/>
      <arg type="i" direction="out" name="height"/>
    </method>
    <signal name="FocusedWindowChanged">
      <arg type="b" name="found"/>
      <arg type="s" name="app_id"/>
      <arg type="s" name="title"/>
      <arg type="i" name="x"/>
      <arg type="i" name="y"/>
      <arg type="i" name="width"/>
      <arg type="i" name="height"/>
    </signal>
    <property name="Version" type="u" access="read"/>
  </interface>
</node>`;

export default class NeruExtension extends Extension {
    enable() {
        this._window = null;
        this._windowSignals = [];
        this._exported = Gio.DBusExportedObject.wrapJSObject(INTERFACE, this);
        this._exported.export(Gio.DBus.session, OBJECT_PATH);
        this._nameId = Gio.bus_own_name(
            Gio.BusType.SESSION, BUS_NAME, Gio.BusNameOwnerFlags.NONE, null, null, null);
        this._focusId = global.display.connect('notify::focus-window', () => this._trackFocus());
        this._trackFocus();
    }

    disable() {
        global.display.disconnect(this._focusId);
        this._untrackWindow();
        Gio.bus_unown_name(this._nameId);
        this._exported.unexport();
        this._exported = null;
    }

    get Version() {
        return 1;
    }

    FocusedWindow() {
        return this._state();
    }

    _state() {
        const window = global.display.focus_window;
        if (!window)
            return [false, '', '', 0, 0, 0, 0];

        const rect = window.get_frame_rect();
        const appId = window.get_wm_class() ?? window.get_gtk_application_id() ?? '';
        return [true, appId, window.get_title() ?? '', rect.x, rect.y, rect.width, rect.height];
    }

    _trackFocus() {
        const window = global.display.focus_window;
        if (window !== this._window) {
            this._untrackWindow();
            this._window = window;
            if (window) {
                for (const name of ['position-changed', 'size-changed', 'notify::title'])
                    this._windowSignals.push(window.connect(name, () => this._emit()));
            }
        }
        this._emit();
    }

    _untrackWindow() {
        if (this._window) {
            for (const id of this._windowSignals)
                this._window.disconnect(id);
        }
        this._windowSignals = [];
        this._window = null;
    }

    _emit() {
        this._exported?.emit_signal('FocusedWindowChanged', new GLib.Variant(STATE_SIGNATURE, this._state()));
    }
}
