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

    $('.change-instance-form').on('submit', function(e) {
        e.preventDefault();
        var wsScheme = "";
        if (scheme == "https:") {
            wsScheme += "ws:"
        } else {
            wsScheme += "ws:"
        }

        var $form = $(this);

        var wsUrl = $form.prop('action').replace(scheme, wsScheme),
            newVal = $form.find('option:selected').val(),
            ws = new WebSocket(wsUrl);

        ws.onopen = function() {
            ws.send(newVal);
            $('#status-msg').show();
            $('.change-instance-form').addClass('disabled-div');
        }

        ws.onerror = function(e) {
            $('.change-instance-form').removeClass('disabled-div');
        }

        ws.onmessage = function(event) {
            var ev = JSON.parse(event.data);
            switch (ev.Status) {
            case "error":
                $('#status-msg')
                    .css("color", '#e51c23')
                    .text(ev.Message);
                $('.change-instance-form').removeClass('disabled-div');
                break;
            case "message":
                var $instanceState = $('#instance-state');
                $instanceState
                .removeClass('btn-primary btn-danger btn-warning btn-default')
                .text(ev.Message)
                .addClass(colorForState(ev.Message));
                break;
            case "success":
                window.location.reload();
            }
        }
    });

    function colorForState(state) {
        switch(state) {
            case "running":
                return "btn-primary";
            case "terminated":
                return "btn-danger";
            case "shutting-down":
                return "btn-danger";
            case "stopped":
                return "btn-warning";
            case "stopping":
                return "btn-warning";
            default:
                return "btn-default";
        }
    }

});
