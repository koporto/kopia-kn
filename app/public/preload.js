const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("kopiaUI", {
  selectDirectory: function (onSelected) {
    ipcRenderer.invoke("select-dir").then((v) => {
      onSelected(v);
    });
  },
  browseDirectory: function (path) {
    ipcRenderer.invoke("browse-dir", path);
  },
});

contextBridge.exposeInMainWorld("knockoutUI", {
  setup: function (payload) {
    return ipcRenderer.invoke("knockout-setup", payload);
  },
  destinationStatus: function () {
    return ipcRenderer.invoke("knockout-destination-status");
  },
});
