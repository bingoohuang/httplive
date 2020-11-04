define(
  [
    "jquery",
    "bootstrap",
    "jsoneditor",
    "httplive/config",
    "knockout",
    "jstree",
    "clipboard",
    "websocket"
  ],
  function(
    $,
    bootstrap,
    jsoneditor,
    config,
    ko,
    jstree,
    clipboard,
    websocket
  ) {
    var webcli = webcli || {};

    webcli.events = {
      treeChanged: "treeChanged",
      treeReady: "treeReady"
    };

    (function($) {
      var o = $({});

      webcli.subscribe = function() {
        o.on.apply(o, arguments);
      };

      webcli.unsubscribe = function() {
        o.off.apply(o, arguments);
      };

      webcli.publish = function() {
        o.trigger.apply(o, arguments);
      };
    })($);

    $(window).bind("keydown", function(event) {
      if (event.ctrlKey || event.metaKey) {
        switch (String.fromCharCode(event.which).toLowerCase()) {
          case "s":
            event.preventDefault();
            break;
        }
      }
    });

    new clipboard(".btnClipboard");

    $("#tree")
      .jstree({
        core: {
          data: {
            check_callback: true,
            cache: false,
            url: config.treePath
          },
          themes: {
            responsive: false,
            variant: "small",
            stripes: true
          },
          multiple: false
        },
        types: {
          root: {
            icon: "glyphicon glyphicon-folder-open",
            valid_children: ["default"]
          },
          default: { icon: "glyphicon glyphicon-flash" }
        },
        plugins: ["state", "types", "unique", "themes", "ui"]
      })
      .on("changed.jstree", function(e, data) {
        if (data.node && data.node.original.type !== "root") {
          var id = data.node.original.id;
          var endpoint = data.node.original.key;
          var originKey = data.node.original.originKey;
          var type = data.node.original.type;
          var context = {
            id: id,
            originKey: originKey,
            type: type,
            endpoint: endpoint
          };
          webcli.publish(webcli.events.treeChanged, context);
        }
      })
      .on("ready.jstree", function() {
        webcli.publish(webcli.events.treeReady, {});
      })
          .on("refresh.jstree", function () {
            let t = $("#tree");
            let selectedID= t.attr('selectedID')
            if (selectedID) {
              // https://stackoverflow.com/questions/8466370/how-to-select-a-specific-node-programmatically
              t.find("li[id=" + selectedID + "] a").click();
              t.attr('selectedID', null);
            }
          });

    webcli.refreshTree = function(id) {
      let t = $("#tree")
      t.attr('selectedID', id)
      t.jstree(true).refresh();
    };

    return webcli;
  }
);
