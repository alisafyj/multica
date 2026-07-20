const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

function plain(value) {
  return JSON.parse(JSON.stringify(value));
}

function node(id, name, type, box, children = []) {
  return {
    id,
    name,
    type,
    visible: true,
    absoluteBoundingBox: box,
    width: box.width,
    height: box.height,
    children,
    exportAsync: async () => new Uint8Array([1, 2, 3]),
  };
}

function hidden(nodeValue) {
  nodeValue.visible = false;
  return nodeValue;
}

function loadPluginContext(pageChildren, selection = []) {
  const pluginPath = path.join(__dirname, "code.js");
  const source = fs.readFileSync(pluginPath, "utf8");
  const context = {
    __html__: "<div></div>",
    console,
    setTimeout,
    clearTimeout,
    Promise,
    Uint8Array,
    Date,
    Math,
    Object,
    Array,
    String,
    Number,
    Boolean,
    figma: {
      mixed: Symbol("mixed"),
      fileKey: "file-key",
      showUI() {},
      clientStorage: { getAsync: async () => "" },
      currentPage: {
        id: "page-1",
        name: "Wallet",
        children: pageChildren,
        selection,
      },
      ui: {
        postMessage() {},
        onmessage: null,
      },
      on() {},
      getNodeByIdAsync: async () => null,
      getImageByHash: () => null,
      openExternal() {},
    },
  };
  vm.createContext(context);
  vm.runInContext(source, context, { filename: pluginPath });
  return context;
}

test("current page summary exposes frames inside a top-level Figma group", () => {
  const frameA = node("1:74", "01 钱包首页-已绑支付宝", "FRAME", { x: 0, y: 0, width: 375, height: 812 });
  const frameB = node("1:174", "02 管理提现账户-已认证", "FRAME", { x: 420, y: 0, width: 375, height: 812 });
  const group = node("4:189", "Group 43", "GROUP", { x: 0, y: 0, width: 795, height: 812 }, [frameA, frameB]);
  const context = loadPluginContext([group]);

  const summary = context.selectionSummary("page");

  assert.deepEqual(plain(summary.active.map((item) => item.name)), [
    "01 钱包首页-已绑支付宝",
    "02 管理提现账户-已认证",
  ]);
});

test("current page export promotes grouped Figma frames into native frames", async () => {
  const frameA = node("1:74", "01 钱包首页-已绑支付宝", "FRAME", { x: 0, y: 0, width: 375, height: 812 }, [
    node("1:75", "标题", "TEXT", { x: 16, y: 20, width: 120, height: 24 }),
  ]);
  const frameB = node("1:174", "02 管理提现账户-已认证", "FRAME", { x: 420, y: 0, width: 375, height: 812 });
  const arrowInstance = node("1:1117", "大-黑箭头10*15备份", "INSTANCE", { x: 820, y: 24, width: 10, height: 15 });
  const group = node("4:189", "Group 43", "GROUP", { x: 0, y: 0, width: 830, height: 812 }, [frameA, frameB, arrowInstance]);
  const context = loadPluginContext([group]);

  const nativeJson = await context.exportNativeJson("page");

  assert.equal(nativeJson.frames.length, 2);
  assert.deepEqual(plain(nativeJson.frames.map((frame) => frame.name)), [
    "01 钱包首页-已绑支付宝",
    "02 管理提现账户-已认证",
  ]);
  assert.equal(nativeJson.frames[0].source.groupName, "Group 43");
  assert.deepEqual(plain(nativeJson.restoreHints.figmaGroups["4-189"].frameIds), ["frame-1", "frame-2"]);
  assert.equal(nativeJson.frames.some((frame) => frame.sourceNodeId === "1:1117"), false);
  assert.equal(nativeJson.layers[nativeJson.frames[0].rootLayerId].frameId, nativeJson.frames[0].id);
});

test("current page summary and export ignore hidden Figma nodes", async () => {
  const hiddenPageFrame = hidden(node("1:1", "隐藏废稿", "FRAME", { x: -500, y: 0, width: 375, height: 812 }));
  const visibleLayer = node("1:11", "标题", "TEXT", { x: 16, y: 20, width: 120, height: 24 });
  const hiddenLayer = hidden(node("1:12", "隐藏标注", "TEXT", { x: 16, y: 60, width: 120, height: 24 }));
  const visibleFrame = node("1:10", "01 钱包首页-已绑支付宝", "FRAME", { x: 0, y: 0, width: 375, height: 812 }, [
    visibleLayer,
    hiddenLayer,
  ]);
  const hiddenGroupedFrame = hidden(node("1:20", "02 钱包首页-废稿", "FRAME", { x: 420, y: 0, width: 375, height: 812 }));
  const group = node("4:189", "Group 43", "GROUP", { x: 0, y: 0, width: 795, height: 812 }, [visibleFrame, hiddenGroupedFrame]);
  const context = loadPluginContext([hiddenPageFrame, group]);

  const summary = context.selectionSummary("page");
  const nativeJson = await context.exportNativeJson("page");

  assert.deepEqual(plain(summary.active.map((item) => item.name)), ["01 钱包首页-已绑支付宝"]);
  assert.deepEqual(plain(nativeJson.frames.map((frame) => frame.name)), ["01 钱包首页-已绑支付宝"]);
  assert.equal(nativeJson.layers["1-11"].name, "标题");
  assert.equal(nativeJson.layers["1-12"], undefined);
  assert.deepEqual(nativeJson.layers[nativeJson.frames[0].rootLayerId].children, ["1-11"]);
});
