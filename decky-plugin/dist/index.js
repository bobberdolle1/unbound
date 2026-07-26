(function (ui, React) {
  'use strict';

  function _interopDefaultLegacy (e) { return e && typeof e === 'object' && 'default' in e ? e : { 'default': e }; }

  var React__default = /*#__PURE__*/_interopDefaultLegacy(React);

  var DefaultContext = {
    color: undefined,
    size: undefined,
    className: undefined,
    style: undefined,
    attr: undefined
  };
  var IconContext = React__default["default"].createContext && React__default["default"].createContext(DefaultContext);

  var __assign = undefined && undefined.__assign || function () {
    __assign = Object.assign || function (t) {
      for (var s, i = 1, n = arguments.length; i < n; i++) {
        s = arguments[i];
        for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p)) t[p] = s[p];
      }
      return t;
    };
    return __assign.apply(this, arguments);
  };
  var __rest = undefined && undefined.__rest || function (s, e) {
    var t = {};
    for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p) && e.indexOf(p) < 0) t[p] = s[p];
    if (s != null && typeof Object.getOwnPropertySymbols === "function") for (var i = 0, p = Object.getOwnPropertySymbols(s); i < p.length; i++) {
      if (e.indexOf(p[i]) < 0 && Object.prototype.propertyIsEnumerable.call(s, p[i])) t[p[i]] = s[p[i]];
    }
    return t;
  };
  function Tree2Element(tree) {
    return tree && tree.map(function (node, i) {
      return React__default["default"].createElement(node.tag, __assign({
        key: i
      }, node.attr), Tree2Element(node.child));
    });
  }
  function GenIcon(data) {
    // eslint-disable-next-line react/display-name
    return function (props) {
      return React__default["default"].createElement(IconBase, __assign({
        attr: __assign({}, data.attr)
      }, props), Tree2Element(data.child));
    };
  }
  function IconBase(props) {
    var elem = function (conf) {
      var attr = props.attr,
        size = props.size,
        title = props.title,
        svgProps = __rest(props, ["attr", "size", "title"]);
      var computedSize = size || conf.size || "1em";
      var className;
      if (conf.className) className = conf.className;
      if (props.className) className = (className ? className + " " : "") + props.className;
      return React__default["default"].createElement("svg", __assign({
        stroke: "currentColor",
        fill: "currentColor",
        strokeWidth: "0"
      }, conf.attr, attr, svgProps, {
        className: className,
        style: __assign(__assign({
          color: props.color || conf.color
        }, conf.style), props.style),
        height: computedSize,
        width: computedSize,
        xmlns: "http://www.w3.org/2000/svg"
      }), title && React__default["default"].createElement("title", null, title), props.children);
    };
    return IconContext !== undefined ? React__default["default"].createElement(IconContext.Consumer, null, function (conf) {
      return elem(conf);
    }) : elem(DefaultContext);
  }

  // THIS FILE IS AUTO GENERATED
  function FaShieldAlt (props) {
    return GenIcon({"tag":"svg","attr":{"viewBox":"0 0 512 512"},"child":[{"tag":"path","attr":{"d":"M466.5 83.7l-192-80a48.15 48.15 0 0 0-36.9 0l-192 80C27.7 91.1 16 108.6 16 128c0 198.5 114.5 335.7 221.5 380.3 11.8 4.9 25.1 4.9 36.9 0C360.1 472.6 496 349.3 496 128c0-19.4-11.7-36.9-29.5-44.3zM256.1 446.3l-.1-381 175.9 73.3c-3.3 151.4-82.1 261.1-175.8 307.7z"}}]})(props);
  }

  // THIS FILE IS AUTO GENERATED
  function BsWifiOff (props) {
    return GenIcon({"tag":"svg","attr":{"fill":"currentColor","viewBox":"0 0 16 16"},"child":[{"tag":"path","attr":{"d":"M10.706 3.294A12.545 12.545 0 0 0 8 3C5.259 3 2.723 3.882.663 5.379a.485.485 0 0 0-.048.736.518.518 0 0 0 .668.05A11.448 11.448 0 0 1 8 4c.63 0 1.249.05 1.852.148l.854-.854zM8 6c-1.905 0-3.68.56-5.166 1.526a.48.48 0 0 0-.063.745.525.525 0 0 0 .652.065 8.448 8.448 0 0 1 3.51-1.27L8 6zm2.596 1.404.785-.785c.63.24 1.227.545 1.785.907a.482.482 0 0 1 .063.745.525.525 0 0 1-.652.065 8.462 8.462 0 0 0-1.98-.932zM8 10l.933-.933a6.455 6.455 0 0 1 2.013.637c.285.145.326.524.1.75l-.015.015a.532.532 0 0 1-.611.09A5.478 5.478 0 0 0 8 10zm4.905-4.905.747-.747c.59.3 1.153.645 1.685 1.03a.485.485 0 0 1 .047.737.518.518 0 0 1-.668.05 11.493 11.493 0 0 0-1.811-1.07zM9.02 11.78c.238.14.236.464.04.66l-.707.706a.5.5 0 0 1-.707 0l-.707-.707c-.195-.195-.197-.518.04-.66A1.99 1.99 0 0 1 8 11.5c.374 0 .723.102 1.021.28zm4.355-9.905a.53.53 0 0 1 .75.75l-10.75 10.75a.53.53 0 0 1-.75-.75l10.75-10.75z"}}]})(props);
  }function BsWifi (props) {
    return GenIcon({"tag":"svg","attr":{"fill":"currentColor","viewBox":"0 0 16 16"},"child":[{"tag":"path","attr":{"d":"M15.384 6.115a.485.485 0 0 0-.047-.736A12.444 12.444 0 0 0 8 3C5.259 3 2.723 3.882.663 5.379a.485.485 0 0 0-.048.736.518.518 0 0 0 .668.05A11.448 11.448 0 0 1 8 4c2.507 0 4.827.802 6.716 2.164.205.148.49.13.668-.049z"}},{"tag":"path","attr":{"d":"M13.229 8.271a.482.482 0 0 0-.063-.745A9.455 9.455 0 0 0 8 6c-1.905 0-3.68.56-5.166 1.526a.48.48 0 0 0-.063.745.525.525 0 0 0 .652.065A8.46 8.46 0 0 1 8 7a8.46 8.46 0 0 1 4.576 1.336c.206.132.48.108.653-.065zm-2.183 2.183c.226-.226.185-.605-.1-.75A6.473 6.473 0 0 0 8 9c-1.06 0-2.062.254-2.946.704-.285.145-.326.524-.1.75l.015.015c.16.16.407.19.611.09A5.478 5.478 0 0 1 8 10c.868 0 1.69.201 2.42.56.203.1.45.07.61-.091l.016-.015zM9.06 12.44c.196-.196.198-.52-.04-.66A1.99 1.99 0 0 0 8 11.5a1.99 1.99 0 0 0-1.02.28c-.238.14-.236.464-.04.66l.706.706a.5.5 0 0 0 .707 0l.707-.707z"}}]})(props);
  }

  function UnboundPanel({ serverAPI }) {
      const [enabled, setEnabled] = React.useState(false);
      const [loading, setLoading] = React.useState(false);
      const [statusText, setStatusText] = React.useState("Checking...");
      React.useEffect(() => {
          fetchStatus();
      }, []);
      const fetchStatus = async () => {
          try {
              const result = await serverAPI.callPluginMethod("status", {});
              if (result.success && result.running !== undefined) {
                  setEnabled(result.running);
                  setStatusText(result.running ? "Bypass Active" : "Bypass Inactive");
              }
              else {
                  setStatusText("Unable to reach daemon");
              }
          }
          catch (e) {
              setStatusText("Error checking status");
          }
      };
      const handleToggle = async (value) => {
          setLoading(true);
          try {
              const result = await serverAPI.callPluginMethod("toggle", { enable: value });
              if (result.success) {
                  setEnabled(value);
                  setStatusText(value ? "Bypass Active" : "Bypass Disabled");
              }
              else {
                  setStatusText(result.error || "Failed to toggle");
              }
          }
          catch (e) {
              setStatusText("Error toggling bypass");
          }
          finally {
              setLoading(false);
          }
      };
      return (h(ui.PanelSection, { title: "Unbound DPI Bypass" },
          h("div", null,
              h(ui.PanelSectionRow, null,
                  h("div", { style: {
                          background: enabled
                              ? "linear-gradient(135deg, rgba(40,160,80,0.15), rgba(40,160,80,0.05))"
                              : "linear-gradient(135deg, rgba(180,60,60,0.15), rgba(180,60,60,0.05))",
                          borderRadius: "8px",
                          padding: "12px 16px",
                          marginBottom: "12px",
                          display: "flex",
                          alignItems: "center",
                          gap: "10px",
                          border: enabled ? "1px solid rgba(40,160,80,0.3)" : "1px solid rgba(180,60,60,0.3)",
                      } },
                      enabled ? (h(BsWifi, { size: 22, color: "#28a050" })) : (h(BsWifiOff, { size: 22, color: "#b43c3c" })),
                      h("div", { style: { flex: 1 } },
                          h("div", { style: { fontWeight: 600, fontSize: "14px" } }, enabled ? "Unbound Active" : "Unbound Inactive"),
                          h("div", { style: { fontSize: "11px", opacity: 0.7, color: "#aaa" } }, statusText)))),
              h(ui.ToggleField, { label: "Enable DPI Bypass", value: enabled, onChange: handleToggle, disabled: loading, description: "Routes traffic through nfqws to bypass DPI/censorship" }),
              h(ui.Button, { onClick: () => fetchStatus(), disabled: loading, style: { marginTop: "8px", width: "100%" } }, loading ? "Refreshing..." : "Refresh Status"))));
  }
  var index = ui.definePlugin((serverAPI) => {
      return {
          title: h("div", null, "Unbound"),
          content: h(UnboundPanel, { serverAPI: serverAPI }),
          icon: h(FaShieldAlt, null),
          onDismount() { },
      };
  });

  return index;

})(DeckyUI, SP_REACT);
