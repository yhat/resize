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

});
