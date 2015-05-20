$(function() {
    if ( $("#awsRegion").length ) {
        $("#awsRegion").on("change", function() {
            var formData = {};
            formData["region"] = this.value;

            $('#instances').addClass('disabled-div');
            $.post("/region", formData)
            .success(function (data) {
                window.location.href = "/";
            })
            .fail(function(xhr, textStatus, errorThrown) {
                alert(xhr.reponseText);
                location.reload();
            });
            return false;
        });
    }

    var scheme = window.location.protocol;

    $('#instance-state').on('click', function(e) {
        e.preventDefault();
    });

    $('#resize').on('submit', function(e) {
        e.preventDefault();
        var wsScheme = "";
        if (scheme == "https:") {
            wsScheme += "ws:"
        } else {
            wsScheme += "ws:"
        }
        var wsUrl = $("#resize").prop('action').replace(scheme, wsScheme),
            newType = $('#change-type').val(),
            ws = new WebSocket(wsUrl),
            resizeSuccessful = true;

        ws.onopen = function() {
            ws.send(newType);
            $('#resizing-message').show();
            $('#resize').addClass('disabled-div');
        }

        ws.onclose = function() {
            if (resizeSuccessful) {
                window.location.reload();
            }
        }

        ws.onerror = function(e) {
            resizeSuccessful = false;
            $('#resize').removeClass('disabled-div');
            console.log(e);
        }

        ws.onmessage = function(event) {
            var ev = JSON.parse(event.data);
            if (ev.Status == "error") {
                $('#resizing-message')
                    .css("color", '#e51c23')
                    .append(ev.Message);
            } else if (ev.Status == "message") {
                var $instanceState = $('#instance-state');
                $instanceState
                    .removeClass('btn-primary btn-danger btn-warning btn-default')
                    .text(ev.Message);
                switch(ev.Message) {
                    case "running":
                        $instanceState.addClass("btn-primary");
                        break;
                    case "terminated":
                        $instanceState.addClass("btn-danger");
                        break;
                    case "shutting-down":
                        $instanceState.addClass("btn-danger");
                        break;
                    case "stopped":
                        $instanceState.addClass("btn-warning");
                        break;
                    case "stopping":
                        $instanceState.addClass("btn-warning");
                        break;
                    default:
                        $instanceState.addClass("btn-default");
                        break;
                }
            }
        }

    })

});
